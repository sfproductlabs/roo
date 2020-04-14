#!/bin/bash
go get github.com/sfproductlabs/roo
#go install github.com/sfproductlabs/roo
CGO_LDFLAGS="-lrocksdb" GODEBUG=netdns=cgo+1  go build -v -o rood github.com/sfproductlabs/roo/v3/roo
sudo docker build -t roo .
