package main

import (
	"context"
	"errors"
)

// ErrCacheMiss is returned when a certificate is not found in cache.
var ErrCacheMiss = errors.New("acme/autocert: certificate cache miss")

// Cache is used by Manager to store and retrieve previously obtained certificates
// and other account data as opaque blobs.
//
// Cache implementations should not rely on the key naming pattern. Keys can
// include any printable ASCII characters, except the following: \/:*?"<>|
type Cache interface {
	// Get returns a certificate data for the specified key.
	// If there's no such key, Get returns ErrCacheMiss.
	Get(ctx context.Context, key string) ([]byte, error)

	// Put stores the data in the cache under the specified key.
	// Underlying implementations may use any data storage format,
	// as long as the reverse operation, Get, results in the original data.
	Put(ctx context.Context, key string, data []byte) error

	// Delete removes a certificate data from the cache under the specified key.
	// If there's no such key in the cache, Delete returns nil.
	Delete(ctx context.Context, key string) error
}

// KvCache implements Cache using a directory on the local filesystem.
// If the directory does not exist, it will be created with 0700 permissions.
type KvCache struct {
}

// Get reads a certificate data from the specified file name.
func (d KvCache) Get(ctx context.Context, name string) ([]byte, error) {
	//name = filepath.Join(string(d), name)
	// var (
	// 	data []byte
	// 	err  error
	// 	done = make(chan struct{})
	// )
	// go func() {
	// 	data, err = ioutil.ReadFile(name)
	// 	close(done)
	// }()
	// select {
	// case <-ctx.Done():
	// 	return nil, ctx.Err()
	// case <-done:
	// }
	// if os.IsNotExist(err) {
	// 	return nil, ErrCacheMiss
	// }
	return nil, nil
}

// Put writes the certificate data to the specified file name.
// The file will be created with 0600 permissions.
func (d KvCache) Put(ctx context.Context, name string, data []byte) error {
	// if err := os.MkdirAll(string(d), 0700); err != nil {
	// 	return err
	// }

	// done := make(chan struct{})
	// var err error
	// go func() {
	// 	defer close(done)
	// 	var tmp string
	// 	if tmp, err = d.writeTempFile(name, data); err != nil {
	// 		return
	// 	}
	// 	defer os.Remove(tmp)
	// 	select {
	// 	case <-ctx.Done():
	// 		// Don't overwrite the file if the context was canceled.
	// 	default:
	// 		newName := filepath.Join(string(d), name)
	// 		err = os.Rename(tmp, newName)
	// 	}
	// }()
	// select {
	// case <-ctx.Done():
	// 	return ctx.Err()
	// case <-done:
	// }
	// return err
	return nil
}

// Delete removes the specified file name.
func (d KvCache) Delete(ctx context.Context, name string) error {
	// name = filepath.Join(string(d), name)
	// var (
	// 	err  error
	// 	done = make(chan struct{})
	// )
	// go func() {
	// 	err = os.Remove(name)
	// 	close(done)
	// }()
	// select {
	// case <-ctx.Done():
	// 	return ctx.Err()
	// case <-done:
	// }
	// if err != nil && !os.IsNotExist(err) {
	// 	return err
	// }
	return nil
}
