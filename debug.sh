#!/bin/bash
CGO_LDFLAGS="-lrocksdb" GODEBUG=netdns=cgo+1 go build -v -o rood github.com/sfproductlabs/roo/v3/roo && GODEBUG=netdns=cgo+1 sudo ./rood ./roo/config.json
