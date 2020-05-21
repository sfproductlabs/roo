/*===----------- kvdocker.go - Distributed Key Value interface in go  -------------===
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
	"net"
	"net/http"
	"regexp"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/lni/goutils/syncutil"
)

func (rt Route) GetKey() string {
	key := HOST_PREFIX + rt.OriginHost
	//Do https by default
	if rt.OriginScheme == "" {
		rt.OriginScheme = "https"
	}
	if rt.OriginScheme == "http" && (rt.OriginPort == "80" || rt.OriginPort == ":http") {
		rt.OriginPort = ""
	}
	if rt.OriginScheme == "https" && (rt.OriginPort == "443" || rt.OriginPort == ":https") {
		rt.OriginPort = ""
	}
	if len(rt.OriginPort) > 0 {
		key = key + ":" + rt.OriginPort
	}
	if len(rt.OriginScheme) == 0 {
		return key + ":https"
	} else {
		return key + ":" + rt.OriginScheme
	}
}

func (rt Route) GetValue() string {
	if len(rt.DestinationScheme) == 0 {
		rt.DestinationScheme = "http"
	}
	return rt.DestinationScheme + "://" + rt.DestinationHost + ":" + rt.DestinationPort
}

func (kvs *KvService) updateFromSwarm(updateRole bool) error {
	routes, err := GetDockerRoutes()
	if updateRole {
		//Add node's role
		rolePrefix := SWARM_WORKER_PREFIX
		if err == nil {
			rolePrefix = SWARM_MANAGER_PREFIX
		}
		action := &KVAction{
			Action: PUT,
			Key:    rolePrefix + kvs.AppConfig.Cluster.Binding,
			Val:    []byte{},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		kvs.execute(ctx, action) //just try and write
	}
	//ACTUALLY WRITE ROUTES TO KV
	if err == nil {
		actions := make([]KVAction, 0, 30)
		for _, rt := range routes {
			actions = append(actions, KVAction{
				Action: PUT,
				Key:    rt.GetKey(),
				Val:    []byte(rt.GetValue()),
			})
		}
		//TODO: Batching from actions
		go func() {
			for _, action := range actions {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_, err = kvs.execute(ctx, &action)
				if err != nil {
					rlog.Warningf("Could not write route: %v to cluster: %v", action, err)
				}
			}
		}()

	}
	return err
}

// The leader raft node delegates any single master node to update the routes every X seconds
func (kvs *KvService) runSwarmWorker() *syncutil.Stopper {
	if kvs.AppConfig.SwarmRefreshSeconds < 1 {
		return nil
	}
	if kvs.AppConfig.SwarmRefreshSeconds < 2 {
		kvs.AppConfig.SwarmRefreshSeconds = 3
	}
	leaderStopper := syncutil.NewStopper()
	leaderStopper.RunWorker(func() {
		ticker := time.NewTicker(time.Duration(kvs.AppConfig.SwarmRefreshSeconds) * time.Second)
		var regex = regexp.MustCompile(`.*\:(.+)`)
		for {
			select {
			case <-ticker.C:
				//Only clean memory in debug mode
				if kvs.AppConfig.Debug {
					runtime.GC()
					debug.FreeOSMemory()
					// m := &runtime.MemStats{}
					// runtime.ReadMemStats(m)
					//rlog.Infof("Current Heap %+v", *m)
				}
				leader, _, _ := kvs.nh.GetLeaderID(kvs.AppConfig.Cluster.Group)
				if leader == kvs.AppConfig.Cluster.NodeID {
					action := &KVAction{
						Action: SCAN,
						Key:    SWARM_MANAGER_PREFIX,
					}
					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(kvs.AppConfig.SwarmRefreshSeconds-1)*time.Second)
					result, err := kvs.nh.SyncRead(ctx, kvs.AppConfig.Cluster.Group, action)
					cancel()
					if err == nil {
						items := result.(map[string][]byte)
						for i, _ := range items {
							matches := regex.FindStringSubmatch(i)
							if len(matches) < 2 {
								continue
							}
							r, _ := http.NewRequest("POST", "http://"+matches[1]+API_PORT+"/roo/"+kvs.AppConfig.ApiVersionString+"/swarm", nil) //TODO: https
							ctx, cancel := context.WithTimeout(r.Context(), time.Duration(4*time.Second))
							r = r.WithContext(ctx)
							client := &http.Client{
								Transport: &http.Transport{
									DisableKeepAlives: true,
									DialContext: (&net.Dialer{
										Timeout:   30 * time.Second,
										KeepAlive: 30 * time.Second,
									}).DialContext,
									TLSHandshakeTimeout:   10 * time.Second,
									ResponseHeaderTimeout: 10 * time.Second,
									ExpectContinueTimeout: 1 * time.Second,
								},
								Timeout: 60 * time.Second,
							}
							resp, err := client.Do(r)
							cancel()
							if resp != nil {
								defer resp.Body.Close()
							}
							client.CloseIdleConnections()
							//Do only once
							if err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 299 {
								rlog.Infof("Swarm Routes Successfully Integrated")
								break
							}
						}
					}
				}
			case <-leaderStopper.ShouldStop():
				return
			}
		}
	})
	return leaderStopper

}
