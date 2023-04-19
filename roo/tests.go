package main

import (
	"context"
	"encoding/json"
	"math/rand"
	"strconv"
	"time"

	"github.com/cockroachdb/pebble"
)

func (configuration *Configuration) testPermissions(kvs *KvService) {
	go func() {
		configuration.TestMutex.Lock()
		// rand.Seed(time.Now().UnixNano())
		// min := 0
		// max := 999999
		// testLog.Infof("[TESTING INTERNAL API 1M READS/WRITES]")
		// db, _ := createDB(testDBDirName + "/_testingPerms")
		// testLog.Infof("[STARTED INTERNAL - WRITE]")
		// for i := 0; i < 1000000; i++ {
		// 	db.db.Set([]byte("_test"+strconv.Itoa(i)), []byte("_test"), &pebble.WriteOptions{Sync: false})
		// }
		// testLog.Infof("[STOPPED INTERNAL - WRITE]")
		// testLog.Infof("[STARTED INTERNAL - READ]")
		// for i := 0; i < 1000000; i++ {
		// 	if _, closer, _ := db.db.Get([]byte("_test" + strconv.Itoa(rand.Intn(max-min+1)+min))); closer != nil {
		// 		closer.Close()
		// 	}
		// }
		// testLog.Infof("[STOPPED INTERNAL - READ]")
		// testLog.Infof("[TESTING EXTERNAL API 1M READS/WRITES]")
		// ctx, cancel := context.WithTimeout(context.Background(), time.Duration(3600*time.Second))
		// defer cancel()
		// cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.ShardID)
		// testLog.Infof("[STARTED EXTERNAL - WRITE]")
		// values := make([]*KVData, 1000000)
		// for i := 0; i < 1000000; i++ {
		// 	values[i] = &KVData{
		// 		Key: "_test" + strconv.Itoa(i),
		// 		Val: []byte("_test"),
		// 	}
		// }
		// kv, _ := json.Marshal(&KVBatch{Batch: values})
		// //TODO: Could use Propose but saturates instead
		// if req, err := kvs.nh.SyncPropose(ctx, cs, kv); err != nil {
		// 	testLog.Errorf("[ERROR] Didn't save %v %v", err, req)
		// }
		// testLog.Infof("[STOPPED EXTERNAL - WRITE]")
		// testLog.Infof("[STARTED EXTERNAL - READ]")
		// for i := 0; i < 1000000; i++ {
		// 	//kv.nh.StaleRead(kv.AppConfig.Cluster.ShardID, "_test"+strconv.Itoa(rand.Intn(max-min+1)+min))
		// 	kvs.nh.SyncRead(ctx, kvs.AppConfig.Cluster.ShardID, "_test"+strconv.Itoa(rand.Intn(max-min+1)+min))
		// }
		// testLog.Infof("[STOPPED EXTERNAL - READ]")
		configuration.TestMutex.Unlock()
	}()

}

func (configuration *Configuration) testKV(kvs *KvService) {
	go func() {
		configuration.TestMutex.Lock()
		time.Sleep(time.Duration(8 * time.Second))
		rand.Seed(time.Now().UnixNano())
		min := 0
		max := 999999
		testLog.Infof("INTERNAL API 1M READS/WRITES")
		db, _ := createDB(testDBDirName + "/_testing")
		testLog.Infof("STARTED INTERNAL - WRITE")
		for i := 0; i < 1000000; i++ {
			db.db.Set([]byte("_test"+strconv.Itoa(i)), []byte("_test"), &pebble.WriteOptions{Sync: false})
		}
		testLog.Infof("STOPPED INTERNAL - WRITE")
		testLog.Infof("STARTED INTERNAL - READ")
		for i := 0; i < 1000000; i++ {
			if _, closer, _ := db.db.Get([]byte("_test" + strconv.Itoa(rand.Intn(max-min+1)+min))); closer != nil {
				closer.Close()
			}
		}
		testLog.Infof("STOPPED INTERNAL - READ")
		testLog.Infof("TESTING EXTERNAL API 1M READS/WRITES")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(3600*time.Second))
		defer cancel()
		cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.ShardID)
		testLog.Infof("STARTED EXTERNAL - WRITE")
		values := make([]*KVData, 1000000)
		for i := 0; i < 1000000; i++ {
			values[i] = &KVData{
				Key: "_test" + strconv.Itoa(i),
				Val: []byte("_test"),
			}
		}
		kv, _ := json.Marshal(&KVBatch{Batch: values})
		//TODO: Could use Propose but saturates instead
		if req, err := kvs.nh.SyncPropose(ctx, cs, kv); err != nil {
			testLog.Errorf("[ERROR] Didn't save %v %v", err, req)
		}
		testLog.Infof("STOPPED EXTERNAL - WRITE")
		testLog.Infof("STARTED EXTERNAL - READ")
		for i := 0; i < 1000000; i++ {
			//kv.nh.StaleRead(kv.AppConfig.Cluster.ShardID, "_test"+strconv.Itoa(rand.Intn(max-min+1)+min))
			kvs.nh.SyncRead(ctx, kvs.AppConfig.Cluster.ShardID, "_test"+strconv.Itoa(rand.Intn(max-min+1)+min))
		}
		testLog.Infof("STOPPED EXTERNAL - READ")
		testLog.Infof("CLEANING UP - START")
		for i := 0; i < 1000000; i++ {
			values[i] = &KVData{
				Key: "_test" + strconv.Itoa(i),
				Val: nil,
			}
		}
		kv, _ = json.Marshal(&KVBatch{Batch: values})
		//TODO: Could use Propose but saturates instead
		if req, err := kvs.nh.SyncPropose(ctx, cs, kv); err != nil {
			testLog.Errorf("[ERROR] Didn't save %v %v", err, req)
		}
		testLog.Infof("CLEANING UP - STOP")
		configuration.TestMutex.Unlock()
	}()
}
