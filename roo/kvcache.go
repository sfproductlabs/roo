// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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

//Certificate Cache
var CertCache = cache.New(3600*time.Second, 90*time.Second)

// Get reads a certificate data from the specified kv.
func (kvs KvService) Get(ctx context.Context, name string) ([]byte, error) {
	if p, found := CertCache.Get(name); found {
		return p.([]byte), nil
	}
	keyname := CACHE_PREFIX + name
	result, err := kvs.nh.SyncRead(ctx, kvs.AppConfig.Cluster.Group, keyname)
	if err != nil {
		rlog.Errorf("SyncRead returned error %v\n", err)
		return nil, err
	}
	//rlog.Infof("[GET] Cache query key: %s, bytes returned: %d\n", keyname, len(result.([]byte)))
	if len(result.([]byte)) == 0 {
		return nil, autocert.ErrCacheMiss
	}
	CertCache.Set(name, result, cache.DefaultExpiration)
	return result.([]byte), nil

}

// Put writes the certificate data to the specified kv.
func (kvs KvService) Put(ctx context.Context, name string, data []byte) error {
	keyname := CACHE_PREFIX + name
	cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.Group)
	kv := &KVAction{
		Key: keyname,
		Val: data,
	}
	kvdata, err := json.Marshal(kv)
	if err != nil {
		rlog.Errorf("[PUT] Cache key: %s, error: %v", kv.Key, err)
	}
	_, err = kvs.nh.SyncPropose(ctx, cs, kvdata)
	CertCache.Set(name, &data, cache.DefaultExpiration)
	return err
}

// Delete removes the specified kv.
func (kvs KvService) Delete(ctx context.Context, name string) error {
	CertCache.Delete(name)
	rlog.Warningf("[kvcache] Delete kv not implemented\n")
	return nil
}
