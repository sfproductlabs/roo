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
	"fmt"
	"time"

	"github.com/lni/goutils/syncutil"
)

func (kvs *KvService) updateFromSwarm() {
	fmt.Println(GetDockerTasks())
	// //Add node's role
	// if kvs.AppConfig.SwarmRole != "" {
	// 	rolePrefix := SWARM_WORKER_PREFIX
	// 	if kvs.AppConfig.SwarmRole == "manager" {
	// 		rolePrefix = SWARM_MANAGER_PREFIX
	// 	}
	// 	action := &KVAction{
	// 		Action: PUT,
	// 		Key:    rolePrefix + kvs.AppConfig.Cluster.Binding,
	// 		Val:    []byte{},
	// 	}
	// 	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	// 	defer cancel()
	// 	kvs.execute(ctx, action)
	// }
}

func (kvs *KvService) runSwarmWorker() *syncutil.Stopper {
	if kvs.AppConfig.SwarmRefreshSeconds < 1 {
		return nil
	}
	leaderStopper := syncutil.NewStopper()
	leaderStopper.RunWorker(func() {
		ticker := time.NewTicker(time.Duration(kvs.AppConfig.SwarmRefreshSeconds) * time.Second) //Update routes in kv from swarm every 60 seconds

		for {
			select {
			case <-ticker.C:
				leader, _, _ := kvs.nh.GetLeaderID(kvs.AppConfig.Cluster.Group)
				if leader == kvs.AppConfig.Cluster.NodeID {
					action := &KVAction{
						Action: SCAN,
						Key:    SWARM_MANAGER_PREFIX,
					}
					ctx, cancel := context.WithTimeout(context.Background(), time.Duration()*time.Second)
					defer cancel()
					result, err := kvs.nh.SyncRead(ctx, kvs.AppConfig.Cluster.Group, action)
					if err == nil {
						items := result.(map[string][]byte)

					}
				}
			case <-leaderStopper.ShouldStop():
				return
			}
		}
	})
	return leaderStopper

}
