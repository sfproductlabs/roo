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
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
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

const (
	PUT RequestType = iota
	GET
)

func parseCommand(msg string) (RequestType, string, string, bool) {
	parts := strings.Split(strings.TrimSpace(msg), " ")
	if len(parts) == 0 || (parts[0] != "put" && parts[0] != "get") {
		return PUT, "", "", false
	}
	if parts[0] == "put" {
		if len(parts) != 3 {
			return PUT, "", "", false
		}
		return PUT, parts[1], parts[2], true
	}
	if len(parts) != 2 {
		return GET, "", "", false
	}
	return GET, parts[1], "", true
}

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
	//TODO: Get Membership from existing nodes
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
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		//v["data"]?
		// raftStopper := syncutil.NewStopper()
		// consoleStopper := syncutil.NewStopper()
		// ch := make(chan string, 16)
		// consoleStopper.RunWorker(func() {
		// 	reader := bufio.NewReader(os.Stdin)
		// 	for {
		// 		s, err := reader.ReadString('\n')
		// 		if err != nil {
		// 			close(ch)
		// 			return
		// 		}
		// 		if s == "exit\n" {
		// 			raftStopper.Stop()
		// 			nh.Stop()
		// 			return
		// 		}
		// 		ch <- s
		// 	}
		// })
		// raftStopper.RunWorker(func() {
		// 	cs := nh.GetNoOPSession(kvs.AppConfig.Cluster.Group)
		// 	for {
		// 		select {
		// 		case v, ok := <-ch:
		// 			if !ok {
		// 				return
		// 			}
		// 			msg := strings.Replace(v, "\n", "", 1)
		// 			// input message must be in the following formats -
		// 			// put key value
		// 			// get key
		// 			rt, key, val, ok := parseCommand(msg)
		// 			if !ok {
		// 				fmt.Fprintf(os.Stderr, "invalid input\n")
		// 				continue
		// 			}
		// 			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		// 			if rt == PUT {
		// 				kv := &KVData{
		// 					Key: key,
		// 					Val: val,
		// 				}
		// 				data, err := json.Marshal(kv)
		// 				if err != nil {
		// 					panic(err)
		// 				}
		// 				_, err = nh.SyncPropose(ctx, cs, data)
		// 				if err != nil {
		// 					fmt.Fprintf(os.Stderr, "SyncPropose returned error %v\n", err)
		// 				}
		// 			} else {
		// 				result, err := nh.SyncRead(ctx, kvs.AppConfig.Cluster.Group, []byte(key))
		// 				if err != nil {
		// 					fmt.Fprintf(os.Stderr, "SyncRead returned error %v\n", err)
		// 				} else {
		// 					fmt.Fprintf(os.Stdout, "query key: %s, result: %s\n", key, result)
		// 				}
		// 			}
		// 			cancel()
		// 		case <-raftStopper.ShouldStop():
		// 			return
		// 		}
		// 	}
		// })
		// go raftStopper.Wait()

		// err := fmt.Errorf("Could not connect to NATS")

		// certFile := kvs.Configuration.Cert
		// keyFile := kvs.Configuration.Key
		// cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		// if err != nil {
		// 	log.Fatalf("[ERROR] Parsing X509 certificate/key pair: %v", err)
		// }

		// rootPEM, err := ioutil.ReadFile(kvs.Configuration.CACert)

		// pool := x509.NewCertPool()
		// ok := pool.AppendCertsFromPEM([]byte(rootPEM))
		// if !ok {
		// 	log.Fatalln("[ERROR] Failed to parse root certificate.")
		// }

		// config := &tls.Config{
		// 	//ServerName:         kvs.Configuration.Hosts[0],
		// 	Certificates:       []tls.Certificate{cert},
		// 	RootCAs:            pool,
		// 	MinVersion:         tls.VersionTLS12,
		// 	InsecureSkipVerify: kvs.Configuration.Secure, //TODO: SECURITY THREAT
		// }

		// if kvs.nc, err = nats.Connect(strings.Join(kvs.Configuration.Hosts[:], ","), nats.Secure(config)); err != nil {
		// 	fmt.Println("[ERROR] Connecting to NATS:", err)
		// 	return err
		// }
		// if kvs.ec, err = nats.NewEncodedConn(kvs.nc, nats.JSON_ENCODER); err != nil {
		// 	fmt.Println("[ERROR] Encoding NATS:", err)
		// 	return err
		// }
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
