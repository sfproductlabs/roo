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

var (
	rlog = logger.GetLogger("roo")
)

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
	//TODO: Join
	//Join existing nodes before bootstrapping
	//Request /roo/api/v1/join from other nodes
	for i, h := range kvs.Configuration.Hosts {

		r, _ := http.NewRequest("GET", "http://"+h+":6299/roo/v1/status", nil)
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(3*time.Second))
		defer cancel()
		r = r.WithContext(ctx)
		client := &http.Client{}
		resp, err := client.Do(r)
		if err != nil {
			fmt.Println(err) //TODO: Change
		} else {
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			fmt.Println("response Body:", string(body), i)
		}

		// var jsonStr = []byte(`{"title":"Buy cheese and bread for breakfast."}`)
		// req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
		// req.Header.Set("X-Custom-Header", "myvalue")
		// req.Header.Set("Content-Type", "application/json")
		// fmt.Println("response Status:", resp.Status)
		// fmt.Println("response Headers:", resp.Header)
		//
		//Requested party runs rs, err = nh.SyncRequestAddNode(ctx, exampleClusterID, nodeID, addr, 0, 3*time.Second), returns same as status
		//
		//Requester then runs StartOnDiskCluster
	}

	//Bootstrap
	//TODO: Exit service if no peers after 5 minutes (will cause a restart and rejoin in swarm)?
	initialMembers := map[uint64]string{kvs.AppConfig.Cluster.NodeID: kvs.AppConfig.Cluster.Binding + KV_PORT}
	alreadyJoined := false
	if err := nh.StartOnDiskCluster(initialMembers, alreadyJoined, NewDiskKV, rc); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add cluster, %v\n", err)
		os.Exit(1)
	}
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
