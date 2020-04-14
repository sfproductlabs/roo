#!/bin/bash
CGO_LDFLAGS="-lrocksdb" GODEBUG=netdns=cgo go build -v -o rood github.com/sfproductlabs/roo/v3/roo && GODEBUG=netdns=cgo sudo ./rood ./roo/config.json
