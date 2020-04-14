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

//////////////////////////////////////// C*
// Connect initiates the primary connection to the range of provided URLs
func (kvs *KvService) connect() error {
	kvs.Configuration.Hosts, _ = net.LookupHost(kvs.AppConfig.Cluster.DNS)
	rlog.Infof("Cluster: Possible Roo IPs: %s\n", kvs.Configuration.Hosts)
	myIPs, _ := getMyIPs(true)
	rlog.Infof("Cluster: My IPs: %s\n", getIPsString(myIPs))
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
		rlog.Infof("Cluster: Binding set to: %s\n", kvs.AppConfig.Cluster.Binding)
	}

	//Now exclude IPs from the hosts (ours or other IPv6)
	tmp := kvs.Configuration.Hosts[:0]
	for i := 0; i < len(kvs.Configuration.Hosts); i++ {
		if kvs.Configuration.Hosts[i] != kvs.AppConfig.Cluster.Binding && net.ParseIP(kvs.Configuration.Hosts[i]) != nil && net.ParseIP(kvs.Configuration.Hosts[i]).To4() != nil {
			tmp = append(tmp, kvs.Configuration.Hosts[i])
		}
	}
	kvs.Configuration.Hosts = tmp
	rlog.Infof("Cluster: Connecting to RAFT: %s\n", kvs.Configuration.Hosts)

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
		rlog.Infof("Attempting to initiate cluster... Waited %d of %d seconds\n", waited, BOOTSTRAP_WAIT_S)
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
	kvs.nh = nh
	kvs.Configuration.Session = kvs
	//Leader finally adds themselves to kv store
	if !alreadyJoined {
		go func() {
			time.Sleep(time.Duration(2) * time.Second) //Usually takes a few seconds for cluster to be ready
			for {
				time.Sleep(time.Duration(1) * time.Second)
				action := &KVAction{
					Action: PUT,
					Key:    PEER_PREFIX + kvs.AppConfig.Cluster.Binding + NODE_POSTFIX,
					Val:    []byte(strconv.FormatUint(kvs.AppConfig.Cluster.NodeID, 10)),
				}
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
				defer cancel()
				if _, err := kvs.execute(ctx, action); err != nil {
					rlog.Infof("Adding leader to kv didn't happen yet: %s\n", err)
					continue
				} else {
					kvs.AppConfig.Cluster.Service.Started = time.Now().UnixNano()
					break
				}

			}
		}()
	} else {
		kvs.AppConfig.Cluster.Service.Started = time.Now().UnixNano()
	}
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
}

//////////////////////////////////////// BIDIRECTIONAL COMMS
func (kvs *KvService) serve(w *http.ResponseWriter, r *http.Request, s *ServiceArgs) error {
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(4*time.Second))
	defer cancel()
	r = r.WithContext(ctx)
	switch s.ServiceType {
	//TODO
	// SERVE_POST_REMOVE = iota
	// SERVE_POST_RESCUE = iota
	case SERVE_POST_JOIN:
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("Bad JS (body)")
		}
		if len(body) > 0 {
			cs := &ClusterStatus{}
			if err := json.Unmarshal(body, cs); err == nil {
				//check the key first, if identical do nothing
				nodestring := strconv.FormatUint(cs.NodeID, 10)
				action := &KVAction{
					Action: GET,
					Key:    PEER_PREFIX + cs.Binding + NODE_POSTFIX,
				}
				if result, err := kvs.execute(r.Context(), action); err == nil {
					if result != nil && string(result.([]byte)) != nodestring {
						//if old/duplicate, remove the old worker, then add node
						if oldnode, err := strconv.ParseUint(string(result.([]byte)), 10, 64); err == nil {
							err = kvs.nh.SyncRequestDeleteNode(r.Context(), cs.Group, oldnode, 0)
							rlog.Infof("[INFO] Pruned node %d: %v, %v", oldnode, cs, err)
						}
					}
				}
				err = kvs.nh.SyncRequestAddNode(r.Context(), cs.Group, cs.NodeID, cs.Binding+KV_PORT, 0)
				rlog.Infof("[INFO] Node requested to join cluster: %v, errors: %v", cs, err)
				if err != nil {
					action = &KVAction{
						Action: PUT,
						Key:    PEER_PREFIX + cs.Binding + NODE_POSTFIX,
						Val:    []byte(nodestring),
					}
					kvs.execute(r.Context(), action)
					writer := *w
					writer.WriteHeader(http.StatusOK)
				} else {
					writer := *w
					writer.WriteHeader(http.StatusBadRequest)
				}
			} else {
				return fmt.Errorf("Bad request (data)")
			}
		} else {
			return fmt.Errorf("Bad request (body)")
		}
		return fmt.Errorf("[ERROR] KV service not implemented %d", s.ServiceType)
	case SERVE_GET_KV:
		action := &KVAction{
			Action: GET,
			Key:    (*s.Values)["key"],
		}
		result, err := kvs.execute(r.Context(), action)
		if err != nil {
			return fmt.Errorf("Could not get key in cluster: %s", err)
		}
		writer := *w
		writer.WriteHeader(http.StatusOK)
		writer.Write(result.([]byte))
		return nil
	case SERVE_GET_KVS:
		action := &KVAction{
			Action: SCAN,
			Key:    (*s.Values)["key"],
		}
		if len(action.Key) > 0 && action.Key[0] == '/' {
			action.Key = action.Key[1:]
		}
		result, err := kvs.execute(r.Context(), action)
		if err != nil {
			return fmt.Errorf("Could not scan cluster: %s", err)
		}
		writer := *w
		json, _ := json.Marshal(map[string]interface{}{"results": result, "query": action.Key}) //Use in javacript: window.atob to decode base64 into json/string if you saved it as that
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		writer.Write(json)
		return nil
	case SERVE_PUT_KV:
		defer r.Body.Close()
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("Could not read posted value")
		}
		action := &KVAction{
			Action: PUT,
			Key:    (*s.Values)["key"],
			Val:    data,
		}
		_, err = kvs.execute(r.Context(), action)
		if err != nil {
			return fmt.Errorf("Could not write key to cluster: %s", err)
		}
		writer := *w
		json, _ := json.Marshal(map[string]interface{}{"ok": true})
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		writer.Write(json)
		return nil
	default:
		return fmt.Errorf("[ERROR] KV service not implemented %d", s.ServiceType)
	}

}

//////////////////////////////////////// WRITES REQUIRE NO FEEDBACK AND ARE ULTRA FAST
func (kvs *KvService) write(w *WriteArgs) error {
	err := fmt.Errorf("Could not write to any kv server in cluster")
	//v := *w.Values
	switch w.WriteType {
	default:
		//TODO: Manually run query via query in config.json
		if kvs.AppConfig.Debug {
			fmt.Printf("UNHANDLED WRITE %s\n", w)
		}
	}

	//TODO: Retries
	return err
}

func (kvs *KvService) execute(ctx context.Context, action *KVAction) (interface{}, error) {
	switch action.Action {
	case SCAN:
		result, err := kvs.nh.SyncRead(ctx, kvs.AppConfig.Cluster.Group, action)
		if err != nil {
			rlog.Errorf("SyncRead returned error %v\n", err)
			return nil, err
		} else {
			rlog.Infof("[SCAN] Execute query key: %s\n", action.Key)
			return result, nil
		}
	case GET:
		result, err := kvs.nh.SyncRead(ctx, kvs.AppConfig.Cluster.Group, action)
		if err != nil {
			rlog.Errorf("SyncRead returned error %v\n", err)
			return nil, err
		} else {
			rlog.Infof("[GET] Execute query key: %s\n", action.Key)
			return result, nil
		}
	case PUT:
		cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.Group)
		kvdata, err := json.Marshal(action)
		if err != nil {
			rlog.Errorf("[PUT] Execute key: %s, error: %v", action.Key, err)
		}
		return kvs.nh.SyncPropose(ctx, cs, kvdata)
	}
	return nil, fmt.Errorf("Method not implemented (execute) in kv %s", action.Action)
}
