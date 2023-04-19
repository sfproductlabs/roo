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
		rand.Seed(time.Now().UnixNano())
		min := 0
		max := 999999
		testLog.Infof("TESTING PERMISSIONS (1M READS/WRITES)")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(3600*time.Second))
		defer cancel()
		testLog.Infof("START PUT PERMISSIONS")
		perms := make([]Permission, max)
		for i := min; i < max; i++ {
			perms[i] = Permission{
				Entity:  TypedString{Val: "dioptre", Rune: 117},
				Context: TypedString{Val: "/folder", Rune: 102},
				Right:   []byte{4},
				Delete:  false,
			}
		}
		//TODO: Could use Propose but saturates instead
		if err := kvs.permiss(ctx, perms); err != nil {
			testLog.Errorf("[ERROR] Didn't save test-permissions %v %v", err)
		}
		testLog.Infof("STOP PUT PERMISSIONS")
		testLog.Infof("START RANDOM READ PERMISSIONS")

		for i := min; i < max; i++ {
			pRequest := &Request{

				User:     TypedString{Val: "dioptre", Rune: 117},
				Entities: []TypedString{{Val: "admins", Rune: 103}, {Val: "marketing", Rune: 103}},
				Resource: TypedString{Val: "/folder1/folder2/wb1", Rune: 102},
				Right:    []byte{4},
			}
			if _, err := kvs.authorize(ctx, *&pRequest); err != nil {
			}

		}
		testLog.Infof("STOP RANDOM READ PERMISSIONS")
		testLog.Infof("CLEANING UP - START")
		perms = make([]Permission, max)
		for i := min; i < max; i++ {
			perms[i] = Permission{
				Entity:  TypedString{Val: "dioptre" + strconv.Itoa(i), Rune: 117},
				Context: TypedString{Val: "/folder" + strconv.Itoa(i), Rune: 102},
				Right:   nil,
				Delete:  true,
			}
		}
		//TODO: Could use Propose but saturates instead
		if err := kvs.permiss(ctx, perms); err != nil {
			testLog.Errorf("[ERROR] Didn't delete test-permissions %v %v", err)
		}
		testLog.Infof("CLEANING UP - STOP")
		configuration.TestMutex.Unlock()
	}()

}

func (configuration *Configuration) testKV(kvs *KvService) {
	go func() {
		configuration.TestMutex.Lock()
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
