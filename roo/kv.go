/*===----------- kv.go - Distributed Key Value interface in go  -------------===
 *
 *
 * This file is licensed under the Apache 2 License. See LICENSE for details.
 *
 *  Copyright (c) 2020 Andrew Grosser. All Rights Reserved.
 *
 *                                     `...
 *                                    yNMMh`
 *                                    dMMMh`
 *                                    dMMMh`
 *                                    dMMMh`
 *                                    dMMMd`
 *                                    dMMMm.
 *                                    dMMMm.
 *                                    dMMMm.               /hdy.
 *                  ohs+`             yMMMd.               yMMM-
 *                 .mMMm.             yMMMm.               oMMM/
 *                 :MMMd`             sMMMN.               oMMMo
 *                 +MMMd`             oMMMN.               oMMMy
 *                 sMMMd`             /MMMN.               oMMMh
 *                 sMMMd`             /MMMN-               oMMMd
 *                 oMMMd`             :NMMM-               oMMMd
 *                 /MMMd`             -NMMM-               oMMMm
 *                 :MMMd`             .mMMM-               oMMMm`
 *                 -NMMm.             `mMMM:               oMMMm`
 *                 .mMMm.              dMMM/               +MMMm`
 *                 `hMMm.              hMMM/               /MMMm`
 *                  yMMm.              yMMM/               /MMMm`
 *                  oMMm.              oMMMo               -MMMN.
 *                  +MMm.              +MMMo               .MMMN-
 *                  +MMm.              /MMMo               .NMMN-
 *           `      +MMm.              -MMMs               .mMMN:  `.-.
 *          /hys:`  +MMN-              -NMMy               `hMMN: .yNNy
 *          :NMMMy` sMMM/              .NMMy                yMMM+-dMMMo
 *           +NMMMh-hMMMo              .mMMy                +MMMmNMMMh`
 *            /dMMMNNMMMs              .dMMd                -MMMMMNm+`
 *             .+mMMMMMN:              .mMMd                `NMNmh/`
 *               `/yhhy:               `dMMd                 /+:`
 *                                     `hMMm`
 *                                     `hMMm.
 *                                     .mMMm:
 *                                     :MMMd-
 *                                     -NMMh.
 *                                      ./:.
 *
 *===----------------------------------------------------------------------===
 */
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/lni/dragonboat/v3"
	"github.com/lni/dragonboat/v3/config"
	"github.com/lni/dragonboat/v3/logger"
)

////////////////////////////////////////
// Interface Implementations
////////////////////////////////////////
type RequestType uint64

//////////////////////////////////////// C*
// Connect initiates the primary connection to the range of provided URLs
func (kvs *KvService) connect() error {
	kvs.Configuration.Hosts, _ = net.LookupHost(kvs.AppConfig.Cluster.DNS)
	myIPs, _ := getMyIPs(true)
	fmt.Printf("Cluster: My IPs: %s\n", getIPsString(myIPs))
	if kvs.AppConfig.Cluster.Binding != "" {
		tempBinding := kvs.AppConfig.Cluster.Binding
		kvs.AppConfig.Cluster.Binding = ""
		for _, ip := range myIPs {
			if ip.String() == tempBinding {
				kvs.AppConfig.Cluster.Binding = tempBinding
				break
			}
		}
	}
	if kvs.AppConfig.Cluster.Binding == "" {
		for _, ip := range myIPs {
			for _, host := range kvs.Configuration.Hosts {
				if ip.String() == host {
					kvs.AppConfig.Cluster.Binding = host
					break
				}
			}
		}
	}
	myip := net.ParseIP(kvs.AppConfig.Cluster.Binding)
	if myip == nil {
		log.Fatalf("[CRITICAL] Cluster: Could not initiate cluster on a service IP")
	} else {
		fmt.Printf("Cluster: Binding set to: %s\n", kvs.AppConfig.Cluster.Binding)
	}

	//Now exclude IPs from the hosts (ours or other IPv6)
	for i := 0; i < len(kvs.Configuration.Hosts); i++ {
		if kvs.Configuration.Hosts[i] == kvs.AppConfig.Cluster.Binding || net.ParseIP(kvs.Configuration.Hosts[i]) == nil || net.ParseIP(kvs.Configuration.Hosts[i]).To4() == nil {
			copy(kvs.Configuration.Hosts[i:], kvs.Configuration.Hosts[i+1:])
			kvs.Configuration.Hosts = kvs.Configuration.Hosts[:len(kvs.Configuration.Hosts)-1]
		}
	}
	fmt.Printf("Cluster: Connecting to RAFT: %s\n", kvs.Configuration.Hosts)

	kvs.AppConfig.Cluster.NodeID = rand.Uint64()

	// https://github.com/golang/go/issues/17393
	if runtime.GOOS == "darwin" {
		signal.Ignore(syscall.Signal(0xd))
	}

	logger.GetLogger("raft").SetLevel(logger.ERROR)
	logger.GetLogger("rsm").SetLevel(logger.WARNING)
	logger.GetLogger("transport").SetLevel(logger.WARNING)
	logger.GetLogger("grpc").SetLevel(logger.WARNING)
	rc := config.Config{
		NodeID:             kvs.AppConfig.Cluster.NodeID,
		ClusterID:          kvs.AppConfig.Cluster.Group,
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    10,
		CompactionOverhead: 5,
	}
	datadir := filepath.Join(
		"cluster-data",
		"roo",
		fmt.Sprintf("node%d", kvs.AppConfig.Cluster.NodeID))

	nhc := config.NodeHostConfig{
		WALDir:         datadir,
		NodeHostDir:    datadir,
		RTTMillisecond: 200,
		RaftAddress:    kvs.AppConfig.Cluster.Binding + KV_PORT,
	}
	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		panic(err)
	}

	apiVersion := "v" + strconv.Itoa(kvs.AppConfig.ApiVersion)
	initialMembers := map[uint64]string{}
	oldest := true
	oldestConfirmed := true
	alreadyJoined := false
	waited := 0
	//Join existing nodes before bootstrapping
	//Request /roo/api/v1/join from other nodes

	for {

		for _, h := range kvs.Configuration.Hosts {
			if h == kvs.AppConfig.Cluster.Binding {
				continue
			}
			r, err := http.NewRequest("GET", "http://"+h+":6299/roo/"+apiVersion+"/status", nil) //TODO: https
			if err != nil {
				rlog.Infof("Bad request to peer (request) %s, %s : %s", h, err)
				continue
			}
			ctx, cancel := context.WithTimeout(r.Context(), time.Duration(4*time.Second))
			defer cancel()
			r = r.WithContext(ctx)
			client := &http.Client{}
			resp, err := client.Do(r)
			if err != nil {
				rlog.Infof("Could not connect to peer %s, %s", h, err)
				oldestConfirmed = false
				continue
			} else {
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					rlog.Infof("Bad response from peer (body) %s, %s : %s", h, err)
					continue
				}
				status := &ClusterStatus{}
				if err := json.Unmarshal(body, status); err != nil {
					rlog.Infof("Bad response from peer (json) %s, %s : %s", h, body, err)
					oldestConfirmed = false
					continue
				}
				if status.Instantiated == kvs.AppConfig.Cluster.Service.Instantiated {
					fmt.Println("[ERROR] Shutting down instance to avoid contention.") //Will auto restart in swarm
					os.Exit(1)
				}
				if status.Instantiated < kvs.AppConfig.Cluster.Service.Instantiated {
					oldest = false
				}
				if status.Started > 0 {
					cs := &ClusterStatus{
						NodeID:  kvs.AppConfig.Cluster.NodeID,
						Group:   kvs.AppConfig.Cluster.Group,
						Binding: kvs.AppConfig.Cluster.Binding,
					}
					if csdata, err := json.Marshal(cs); err != nil {
						rlog.Infof("Bad request to peer (json) %s, %s : %s", h, cs, err)
						continue
					} else {
						req, err := http.NewRequest("POST", "http://"+h+":6299/roo/"+apiVersion+"/join", bytes.NewBuffer(csdata))
						if err != nil {
							rlog.Infof("Bad request to peer (request) %s, %s : %s", h, cs, err)
							continue
						}
						_, err = (&http.Client{}).Do(req)
						if err == nil {
							initialMembers[status.NodeID] = status.Binding + KV_PORT
							alreadyJoined = true
						}
					}
				}
			}
		}
		waited = waited + 1
		if alreadyJoined {
			break
		}
		if oldest && oldestConfirmed {
			alreadyJoined = false
			break
		}
		if waited >= BOOTSTRAP_WAIT_S {
			if oldest {
				alreadyJoined = false
			}
			break
		}
		fmt.Printf("Attempting to initiate cluster... Waited %d of %d seconds/n", waited, BOOTSTRAP_WAIT_S)
		time.Sleep(time.Duration(1) * time.Second)
	}

	if len(initialMembers) == 0 {
		initialMembers[kvs.AppConfig.Cluster.NodeID] = kvs.AppConfig.Cluster.Binding + KV_PORT
		alreadyJoined = false
	}

	//Bootstrap
	//TODO: Exit service if no peers after 5 minutes (will cause a restart and rejoin in swarm)?
	if err := nh.StartOnDiskCluster(initialMembers, alreadyJoined, NewDiskKV, rc); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to add cluster, %v\n", err)
		os.Exit(1)
	}
	kvs.AppConfig.Cluster.Service.Started = time.Now().UnixNano()
	kvs.nh = nh
	kvs.Configuration.Session = kvs
	return nil
}

//////////////////////////////////////// C*
// Close will terminate the session to the backend, returning error if an issue arises
func (i *KvService) close() error {
	return fmt.Errorf("[ERROR] KV close not implemented")
}

func (i *KvService) listen() error {
	//TODO: Listen for KV triggers
	return fmt.Errorf("[ERROR] KV listen not implemented")
}

func (i *KvService) auth(s *ServiceArgs) error {
	return fmt.Errorf("Not implemented")
	// //TODO: AG implement JWT
	// //TODO: AG implement creds (check domain level auth)
	// if *s.Values == nil {
	// 	return fmt.Errorf("User not provided")
	// }
	// uid := (*s.Values)["uid"]
	// if uid == "" {
	// 	return fmt.Errorf("User ID not provided")
	// }
	// password := (*s.Values)["password"]
	// if password == "" {
	// 	return fmt.Errorf("User pass not provided")
	// }
}

func (kvs *KvService) serve(w *http.ResponseWriter, r *http.Request, s *ServiceArgs) error {
	switch s.ServiceType {
	case SERVE_GET_PING:
		json, _ := json.Marshal(map[string]interface{}{"ping": (*s.Values)["pong"]})
		(*w).Header().Set("Content-Type", "application/json")
		(*w).WriteHeader(http.StatusOK)
		(*w).Write(json)
		return nil
	case SERVE_POST_JOIN:
		//check the key first, if identical do nothing
		//if old, remove the old worker, then add node
		//
		//Requested party runs rs, err = nh.SyncRequestAddNode(ctx, exampleClusterID, nodeID, addr, 0, 3*time.Second), returns same as status
		//
		//Requester then runs StartOnDiskCluster
		return fmt.Errorf("[ERROR] KV service not implemented %d", s.ServiceType)
	default:
		return fmt.Errorf("[ERROR] KV service not implemented %d", s.ServiceType)
	}

}

//////////////////////////////////////// C*
func (kvs *KvService) write(w *WriteArgs) error {
	err := fmt.Errorf("Could not write to any kv server in cluster")
	//v := *w.Values
	switch w.WriteType {
	case WRITE_PUT_KV:
		//ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		//v["data"]?

		return err
	default:
		//TODO: Manually run query via query in config.json
		if kvs.AppConfig.Debug {
			fmt.Printf("UNHANDLED %s\n", w)
		}
	}

	//TODO: Retries
	return err
}
