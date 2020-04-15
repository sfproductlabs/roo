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
)

// ErrCacheMiss is returned when a certificate is not found in cache.
//var ErrCacheMiss = errors.New("Cache miss")

// Get reads a certificate data from the specified kv.
func (kvs KvService) Get(ctx context.Context, name string) ([]byte, error) {
	result, err := kvs.nh.SyncRead(ctx, kvs.AppConfig.Cluster.Group, []byte(name))
	if err != nil {
		rlog.Errorf("SyncRead returned error %v\n", err)
		return nil, err
	}
	rlog.Infof("[GET] Cache query key: %s, result: %s\n", name, result)
	if len(result.([]byte)) == 0 {
		return nil, autocert.ErrCacheMiss
	}
	return result.([]byte), nil

}

// Put writes the certificate data to the specified kv.
func (kvs KvService) Put(ctx context.Context, name string, data []byte) error {
	cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.Group)
	kv := &KVAction{
		Key: name,
		Val: data,
	}
	kvdata, err := json.Marshal(kv)
	if err != nil {
		rlog.Errorf("[PUT] Cache key: %s, error: %v", kv.Key, err)
	}
	_, err = kvs.nh.SyncPropose(ctx, cs, kvdata)
	return err
}

// Delete removes the specified kv.
func (kvs KvService) Delete(ctx context.Context, name string) error {
	rlog.Warningf("[kvcache] Delete not implemented\n")
	return nil
}
