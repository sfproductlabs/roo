/*===----------- kvwatcher.go - Distributed Key Value interface in go  -------------===
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
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/lni/goutils/syncutil"
	"github.com/patrickmn/go-cache"
)

// The leader raft node delegates any single master node to update the routes every X seconds
func (kvs *KvService) runRaftWatcher() *syncutil.Stopper {
	pingCache := cache.New(time.Duration(BOOTSTRAP_WAIT_S)*time.Second, 1*time.Second)
	leaderStopper := syncutil.NewStopper()
	leaderStopper.RunWorker(func() {
		ticker := time.NewTicker(time.Duration(1) * time.Second)
		for {
			select {
			case <-ticker.C:
				leader, _, _ := kvs.nh.GetLeaderID(kvs.AppConfig.Cluster.Group)
				if leader == kvs.AppConfig.Cluster.NodeID {
					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(4)*time.Second)
					membership, cerr := kvs.nh.SyncGetClusterMembership(ctx, kvs.AppConfig.Cluster.Group)
					ctx.Done()
					cancel()
					if cerr == nil {
						for nodeid, node := range membership.Nodes {
							host, _, _ := net.SplitHostPort(node)
							r, _ := http.NewRequest("GET", "http://"+host+API_PORT+"/roo/"+kvs.AppConfig.ApiVersionString+"/status", nil) //TODO: https
							client := &http.Client{
								Transport: &http.Transport{
									DisableKeepAlives: true,
									Dial:              TimeoutDialer(12*time.Second, 12*time.Second),
								},
								Timeout: 12 * time.Second,
							}
							resp, err := client.Do(r)
							if resp != nil {
								ioutil.ReadAll(resp.Body)
								resp.Body.Close()
							}
							client.CloseIdleConnections()
							if err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 299 {
								pingCache.Set(host, time.Now().UnixNano(), cache.DefaultExpiration)
								continue
							} else {
								rlog.Infof("[PING] Node host not repsonding %s", host)
								if pinged, found := pingCache.Get(host); found {
									if (time.Now().UnixNano() - pinged.(int64)) > int64(BOOTSTRAP_WAIT_S)*10^9 {
										rs, err := kvs.nh.RequestDeleteNode(kvs.AppConfig.Cluster.Group, nodeid, 0, 10*time.Second)
										if err != nil {
											rlog.Warningf("[WARNING] Failed to prune found node %s: %v, %v", host, rs, err)
										} else {
											rlog.Infof("[INFO] Pruned found node %s: %v", host, rs)
										}
									}
								} else {
									rs, err := kvs.nh.RequestDeleteNode(kvs.AppConfig.Cluster.Group, nodeid, 0, 10*time.Second)
									if err != nil {
										rlog.Warningf("[WARNING] Failed to prune missing node %s: %v, %v", host, rs, err)
									} else {
										rlog.Infof("[INFO] Pruned missing node %s: %v", host, rs)
									}
								}
							}
						}
					} else {
						rlog.Infof("[INFO] Couldn't get membership in watcher.")
					}
				}
			case <-leaderStopper.ShouldStop():
				return
			}
		}
	})
	return leaderStopper

}
