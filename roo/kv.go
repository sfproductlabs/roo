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

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
)

// ////////////////////////////////////// C*
// Connect initiates the primary connection to the range of provided URLs
func (kvs *KvService) connect() error {

	myIPs, _ := getMyIPs(true)
	myIPstrings := getIPsString(myIPs)
	rlog.Infof("Cluster: My IPs: %s\n", myIPstrings)
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
	tmp := make([]string, 0)
	for i := 0; i < len(kvs.Configuration.Hosts); i++ {
		if kvs.Configuration.Hosts[i] != kvs.AppConfig.Cluster.Binding && net.ParseIP(kvs.Configuration.Hosts[i]) != nil && net.ParseIP(kvs.Configuration.Hosts[i]).To4() != nil {
			tmp = append(tmp, kvs.Configuration.Hosts[i])
		}
	}
	kvs.Configuration.Hosts = tmp
	rlog.Infof("Cluster: Connecting to RAFT: %s\n", kvs.Configuration.Hosts)

	kvs.AppConfig.Cluster.ReplicaID = uint64(rand.Intn(65534) + 1)
	//kvs.AppConfig.Cluster.ShardID = rand.Uint64()

	// https://github.com/golang/go/issues/17393
	if runtime.GOOS == "darwin" {
		signal.Ignore(syscall.Signal(0xd))
	}

	rc := config.Config{
		ReplicaID:          kvs.AppConfig.Cluster.ReplicaID, //ReplicaID
		ShardID:            kvs.AppConfig.Cluster.ShardID,   //Group
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    10,
		CompactionOverhead: 5,
	}
	if kvs.AppConfig.DataDirectoryRoot == "" {
		kvs.AppConfig.DataDirectoryRoot = "cluster-data"
	}
	datadir := filepath.Join(
		kvs.AppConfig.DataDirectoryRoot,
		"roo",
		fmt.Sprintf("node%d", kvs.AppConfig.Cluster.ReplicaID))

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

	initialMembers := map[uint64]string{kvs.AppConfig.Cluster.ReplicaID: kvs.AppConfig.Cluster.Binding + KV_PORT}
	olderThan := 0
	bootstrap := false
	readyToJoin := false
	waited := 0
	//Join existing nodes before bootstrapping
	//Request /roo/api/v1/join from other nodes

	for {
		olderThan = 0
		for _, h := range kvs.Configuration.Hosts {
			fmt.Println(h)
			if h == kvs.AppConfig.Cluster.Binding ||
				//sort.Search(len(myIPstrings), func(i int) bool { return myIPstrings[i] == h }) > 0 ||
				net.ParseIP(h).To4() == nil {
				continue
			}
		getNodeStatus:
			r, _ := http.NewRequest("GET", "http://"+h+API_PORT+"/roo/"+kvs.AppConfig.ApiVersionString+"/status", nil) //TODO: https
			ctx, cancel := context.WithTimeout(r.Context(), time.Duration(4*time.Second))
			defer cancel()
			r = r.WithContext(ctx)
			client := &http.Client{}
			resp, err := client.Do(r)
			if resp != nil {
				defer resp.Body.Close()
			}
			if err != nil || resp == nil {
				rlog.Infof("Could not connect to peer %s, %s", h, err)
				time.Sleep(time.Duration(1) * time.Second)
				waited = waited + 1
				if waited >= BOOTSTRAP_WAIT_S {
					fmt.Println("[ERROR] Could not confirm status from initial peers.")
					os.Exit(1)
				}
				goto getNodeStatus
			} else {
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					rlog.Infof("Bad response from peer (body) %s, %s : %s", h, err)
					continue
				}
				status := &ClusterStatus{}
				if err := json.Unmarshal(body, status); err != nil {
					rlog.Infof("Bad response from peer (json) %s, %s : %s", h, body, err)
					continue
				}
				//TODO: Check if the node id is the same as the others
				//Then reboot

				if status.Instantiated == kvs.AppConfig.Cluster.Service.Instantiated {
					fmt.Println("[ERROR] Shutting down instance to avoid contention.") //Will auto restart in swarm
					os.Exit(1)
				}
				if status.Instantiated > kvs.AppConfig.Cluster.Service.Instantiated {
					olderThan = olderThan + 1
				}
				initialMembers[status.ReplicaID] = status.Binding + KV_PORT
				if status.Started > 0 {
					checkedBootstrapped := 0
				checkBootstrap:
					//Check to see if cluster already bootstrapped (YES: join, NO: bootstrap)
					r, _ := http.NewRequest("GET", "http://"+h+API_PORT+"/roo/"+kvs.AppConfig.ApiVersionString+"/kv/"+ROO_STARTED, nil)
					ctx, cancel := context.WithTimeout(r.Context(), time.Duration(4*time.Second))
					defer cancel()
					r = r.WithContext(ctx)
					resp, err := (&http.Client{}).Do(r)
					body := []byte("false")
					if resp != nil {
						defer resp.Body.Close()
						body, err = ioutil.ReadAll(resp.Body)
					}
					if resp != nil && err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 299 && string(body) == "true" {
						rlog.Infof("[[DISCOVERED BOOTSTRAPPED CLUSTER]]")
						bootstrap = false
						cs := &ClusterStatus{
							ReplicaID: kvs.AppConfig.Cluster.ReplicaID,
							ShardID:   kvs.AppConfig.Cluster.ShardID,
							Binding:   kvs.AppConfig.Cluster.Binding,
						}
						if csdata, err := json.Marshal(cs); err != nil {
							rlog.Infof("Bad request to peer (json) %s, %s : %s", h, cs, err)
							continue
						} else {
							time.Sleep(time.Duration(1) * time.Second)
							req, err := http.NewRequest("POST", "http://"+h+API_PORT+"/roo/"+kvs.AppConfig.ApiVersionString+"/join", bytes.NewBuffer(csdata))
							if err != nil {
								rlog.Infof("Bad request to peer (request) %s, %s : %s", h, cs, err)
								continue
							}
							ctx, cancel := context.WithTimeout(req.Context(), time.Duration(30*time.Second))
							defer cancel()
							req = req.WithContext(ctx)
							resp, err := (&http.Client{}).Do(req)
							if resp != nil {
								defer resp.Body.Close()
							}
							if err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 299 {
								initialMembers = map[uint64]string{}
								readyToJoin = true
								break
							}
							rlog.Warningf("[WARNING] Request to join failed, status-code: %d err: %v", resp, err)
						}
					} else {
						if checkedBootstrapped > -1 {
							rlog.Infof("[[COULDN'T FIND BOOTSTRAPPED CLUSTER]]")
							bootstrap = true
							continue
						} else {
							checkedBootstrapped = checkedBootstrapped + 1
							time.Sleep(time.Duration(4) * time.Second)
							goto checkBootstrap
						}
					}

				}
			}
		}
		waited = waited + 1
		if bootstrap {
			rlog.Infof("[[BOOTSTRAPPING]]\n")
			break
		}
		if olderThan == len(kvs.Configuration.Hosts) {
			rlog.Infof("[[OLDEST]]\n")
			bootstrap = true
			break
		}
		if readyToJoin {
			rlog.Infof("[[PEERING]]\n")
			bootstrap = false
			break
		}
		if waited >= BOOTSTRAP_WAIT_S {
			fmt.Println("[ERROR] Could not confirm node is the oldest before time ran out.")
			os.Exit(1)
		}
		rlog.Infof("Attempting to initiate cluster... Waited %d of %d seconds\n", waited, BOOTSTRAP_WAIT_S)
		time.Sleep(time.Duration(1) * time.Second)
	}

	//Begin cluster
rejoin:
	if err := nh.StartOnDiskReplica(initialMembers, !bootstrap, NewDiskKV, rc); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to add cluster, %v, members: %v\n", err, initialMembers)
		if bootstrap {
			time.Sleep(time.Duration(1) * time.Second)
			goto rejoin
		}
		os.Exit(1)
	}

	go func() {
		for {
			kvs.AppConfig.Cluster.Service.Started = time.Now().UnixNano()
			//Add node to kv store
			{
				action := &KVAction{
					Action: PUT,
					Data: &KVData{
						Key: PEER_PREFIX + kvs.AppConfig.Cluster.Binding + NODE_POSTFIX,
						Val: []byte(strconv.FormatUint(kvs.AppConfig.Cluster.ReplicaID, 10)),
					},
				}
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if _, err := kvs.execute(ctx, action); err != nil {
					rlog.Infof("Adding node to kv didn't happen yet: %s probably still waiting for cluster\n", err)
					time.Sleep(time.Duration(7) * time.Second)
					continue
				}
			}
			go func() {
				//Delay started to allow bootstrapped nodes to join in time
				//Implies we can't scale for another BOOTSTRAP_WAIT_S/2
				for {
					time.Sleep(time.Duration(BOOTSTRAP_WAIT_S/2) * time.Second)
					started := &KVAction{
						Action: PUT,
						Data: &KVData{
							Key: ROO_STARTED,
							Val: []byte(strconv.FormatBool(true)),
						},
					}
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if _, err := kvs.execute(ctx, started); err != nil {
						rlog.Infof("Adding roo.started value failed\n", err)
						time.Sleep(time.Duration(7) * time.Second)
						continue
					} else {
						rlog.Infof("[[ROO RE/CONFIRMED STARTED]]\n")
					}
					break
				}
			}()
			rlog.Infof("[[CREATED NODE]]\n")
			//Update the routes once per node on boot
			if kvs.AppConfig.Swarm {
				kvs.updateFromSwarm(true)
				kvs.swarmUpdater = kvs.runSwarmWorker()
				kvs.raftWatcher = kvs.runRaftWatcher()
			}
			break
		}
	}()
	kvs.nh = nh
	kvs.Configuration.Session = kvs
	return nil
}

// ////////////////////////////////////// C*
// Close will terminate the session to the backend, returning error if an issue arises
func (i *KvService) close() error {
	return fmt.Errorf("[ERROR] KV close not implemented")
}

func (i *KvService) listen() error {
	//TODO: Listen for KV triggers
	return fmt.Errorf("[ERROR] KV listen not implemented")
}

func (i *KvService) auth(s *ServiceArgs) error {
	return fmt.Errorf("[ERROR] KV auth not implemented")
}

// ////////////////////////////////////// BIDIRECTIONAL COMMS
func (kvs *KvService) serve(w *http.ResponseWriter, r *http.Request, s *ServiceArgs) error {
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(12*time.Second))
	defer cancel()
	r = r.WithContext(ctx)
	switch s.ServiceType {
	case SERVE_POST_PERM:
		if data, err := ioutil.ReadAll(r.Body); err != nil {
			return fmt.Errorf("Could not parse body: %s", err)
		} else {
			pRequest := &Request{}
			if err := json.Unmarshal(data, &pRequest); err != nil {
				return fmt.Errorf("Could not parse object: %s", err)
			}
			if ok, err := kvs.authorize(r.Context(), *&pRequest); err != nil {
				return fmt.Errorf("Could not authorize: %s", err)
			} else if ok {
				(*w).WriteHeader(http.StatusOK)
			} else {
				(*w).WriteHeader(http.StatusForbidden)
			}
		}
		return nil
	case SERVE_PUT_PERM:
		if data, err := ioutil.ReadAll(r.Body); err != nil {
			return fmt.Errorf("Could not parse body: %s", err)
		} else {
			perms := &[]Permisson{}
			if err := json.Unmarshal(data, &perms); err != nil {
				return fmt.Errorf("Could not parse object: %s", err)
			}
			if err := kvs.permiss(r.Context(), *perms); err != nil {
				return fmt.Errorf("Could not set permission: %s", err)
			}
		}
		(*w).WriteHeader(http.StatusOK)
		return nil
	case SERVE_GET_KV:
		action := &KVAction{
			Action: GET,
			Data: &KVData{
				Key: (*s.Values)["key"],
			},
		}
		result, err := kvs.execute(r.Context(), action)
		if err != nil {
			return fmt.Errorf("Could not get key in cluster: %s", err)
		}
		(*w).WriteHeader(http.StatusOK)
		(*w).Write(result.([]byte))
		return nil
	case SERVE_PUT_KV:
		defer r.Body.Close()
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("Could not read posted value")
		}
		action := &KVAction{
			Action: PUT,
			Data: &KVData{
				Key: (*s.Values)["key"],
				Val: data,
			},
		}
		_, err = kvs.execute(r.Context(), action)
		if err != nil {
			return fmt.Errorf("Could not write key to cluster: %s", err)
		}

		json, _ := json.Marshal(map[string]interface{}{"ok": true})
		(*w).Header().Set("Content-Type", "application/json")
		(*w).WriteHeader(http.StatusOK)
		(*w).Write(json)
		return nil
	case SERVE_GET_KVS:
		action := &KVAction{
			Action: SCAN,
			Data: &KVData{
				Key: (*s.Values)["key"],
			},
		}
		if len(action.Data.Key) > 0 && action.Data.Key[0] == '/' {
			action.Data.Key = action.Data.Key[1:]
		}
		result, err := kvs.execute(r.Context(), action)
		if err != nil {
			return fmt.Errorf("Could not scan cluster: %s", err)
		}
		json, _ := json.Marshal(map[string]interface{}{"results": result, "query": action.Data.Key}) //Use in javacript: window.atob to decode base64 into json/string if you saved it as that
		(*w).Header().Set("Content-Type", "application/json")
		(*w).WriteHeader(http.StatusOK)
		(*w).Write(json)
		return nil
		//TODO
	case SERVE_POST_REMOVE:
		return fmt.Errorf("Remove not implemented")
	case SERVE_POST_RESCUE:
		return fmt.Errorf("Rescue not implemented")
	case SERVE_POST_SWARM:
		if err := kvs.updateFromSwarm(false); err != nil {
			return err
		}
		(*w).WriteHeader(http.StatusOK)
		(*w).Write([]byte{})
		return nil
	case SERVE_POST_JOIN:
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("Bad JS (body)")
		}
		if len(body) > 0 {
			cs := &ClusterStatus{}
			if err := json.Unmarshal(body, cs); err == nil {
				var rs *dragonboat.RequestState
				//check the key first, if identical do nothing
				nodestring := strconv.FormatUint(cs.ReplicaID, 10)
				action := &KVAction{
					Action: GET,
					Data: &KVData{
						Key: PEER_PREFIX + cs.Binding + NODE_POSTFIX,
					},
				}
				if result, err := kvs.execute(r.Context(), action); err == nil {
					if result != nil && string(result.([]byte)) != nodestring {
						//if old/duplicate, remove the old worker, then add node
						if oldnode, err := strconv.ParseUint(string(result.([]byte)), 10, 64); err == nil {
							rs, err = kvs.nh.RequestDeleteReplica(cs.ShardID, oldnode, 0, 3*time.Second)
							rlog.Infof("[INFO] Pruned node %d: %v, %v", oldnode, cs, err)
							if err != nil {
								return err
							}
						}
					}
				}
				rs, err = kvs.nh.RequestAddReplica(cs.ShardID, cs.ReplicaID, cs.Binding+KV_PORT, 0, 3*time.Second)
				rlog.Infof("[INFO] Node requested to join cluster: %v, errors: %v", cs, err)
				select {
				case res := <-rs.CompletedC:
					if res.Completed() {
						action = &KVAction{
							Action: PUT,
							Data: &KVData{
								Key: PEER_PREFIX + cs.Binding + NODE_POSTFIX,
								Val: []byte(nodestring),
							},
						}
						if _, err := kvs.execute(r.Context(), action); err != nil {
							return err
						} else {
							(*w).WriteHeader(http.StatusOK)
							return nil
						}
					} else {
						return fmt.Errorf("Membership add failed %v\n", res)
					}
				}
			} else {
				return fmt.Errorf("Bad request (data)")
			}
		} else {
			return fmt.Errorf("Bad request (body)")
		}
	default:
		return fmt.Errorf("[ERROR] KV service not implemented %d", s.ServiceType)
	}

}

// ////////////////////////////////////// WRITES REQUIRE NO FEEDBACK AND ARE ULTRA FAST
func (kvs *KvService) write(w *WriteArgs) error {
	err := fmt.Errorf("Could not write to any kv server in cluster")
	//v := *w.Values
	switch w.WriteType {
	default:
		//TODO: Manually run query via query in config.json
		if kvs.AppConfig.Debug {
			fmt.Printf("UNHANDLED WRITE %v\n", w)
		}
	}

	//TODO: Retries
	return err
}

func (kvs *KvService) execute(ctx context.Context, action *KVAction) (interface{}, error) {
	if action.Data == nil {
		return nil, fmt.Errorf("Key missing %s", action.Action)
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()
	switch action.Action {
	case SCAN:
		result, err := kvs.nh.SyncRead(cctx, kvs.AppConfig.Cluster.ShardID, action)
		if err != nil {
			rlog.Errorf("SyncRead returned error %v\n", err)
			return nil, err
		} else {
			rlog.Infof("[SCAN] Execute query key: %s\n", action.Data.Key)
			return result, nil
		}
	case GET:
		result, err := kvs.nh.SyncRead(cctx, kvs.AppConfig.Cluster.ShardID, action)
		if err != nil {
			rlog.Errorf("SyncRead returned error %v\n", err)
			return nil, err
		} else {
			rlog.Infof("[GET] Execute query key: %s\n", action.Data.Key)
			return result, nil
		}
	case PUT:
		if action.Data.Key == "" {
			return nil, fmt.Errorf("Key missing %s", action.Action)
		}
		cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.ShardID)
		kvdata, err := json.Marshal(action)
		if err != nil {
			rlog.Errorf("[PUT] Execute key: %s, error: %v", action.Data.Key, err)
		}
		return kvs.nh.SyncPropose(cctx, cs, kvdata)
	}
	return nil, fmt.Errorf("Method not implemented (execute) in kv %s", action.Action)
}
