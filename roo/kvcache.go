// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"context"
	"encoding/json"

	"golang.org/x/crypto/acme/autocert"

	"time"

	"github.com/patrickmn/go-cache"
)

// Certificate Cache
var CertCache = cache.New(3600*time.Second, 90*time.Second)

// Get reads a certificate data from the specified kv.
func (kvs KvService) Get(ctx context.Context, name string) ([]byte, error) {
	if p, found := CertCache.Get(name); found {
		return p.([]byte), nil
	}
	keyname := CACHE_PREFIX + name
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()
	result, err := kvs.nh.SyncRead(cctx, kvs.AppConfig.Cluster.ShardID, keyname)
	if err != nil {
		rlog.Errorf("SyncRead returned error %v\n", err)
		return nil, err
	}
	if len(result.([]byte)) == 0 {
		//TODO: Add malware tracking tool here
		rlog.Infof("[GET] Cache miss, key: %s\n", keyname)
		return nil, autocert.ErrCacheMiss
	}
	CertCache.Set(name, result, cache.DefaultExpiration)
	return result.([]byte), nil

}

// Put writes the certificate data to the specified kv.
func (kvs KvService) Put(ctx context.Context, name string, data []byte) error {
	keyname := CACHE_PREFIX + name
	cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.ShardID)
	kv := &KVAction{
		Key: keyname,
		Val: data,
	}
	kvdata, err := json.Marshal(kv)
	if err != nil {
		rlog.Errorf("[PUT] Cache key: %s, error: %v", kv.Key, err)
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()
	_, err = kvs.nh.SyncPropose(cctx, cs, kvdata)
	if err == nil {
		CertCache.Set(name, &data, cache.DefaultExpiration)
	}
	return err
}

// Delete removes the specified kv.
func (kvs KvService) Delete(ctx context.Context, name string) error {
	CertCache.Delete(name)
	return kvs.Put(ctx, name, []byte{})
	//TODO: Complete delete implementation, bit hacky atm
}
