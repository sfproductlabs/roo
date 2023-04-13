#!/bin/bash
go get github.com/sfproductlabs/roo
#go install github.com/sfproductlabs/roo
oldver=`grep -oP '(?<=Roo\.\ Version\ ).*(?=\")' ./roo/roo.go`
newver=`expr $oldver + 1`
sed -i "s/Roo\.\ Version\ $oldver/Roo\.\ Version\ $newver/g" "./roo/roo.go"
sudo rm -rf cluster-data
#CGO_LDFLAGS="-lrocksdb" go build -v -o rood github.com/sfproductlabs/roo/v3/roo
go build -v -o rood github.com/sfproductlabs/roo/v4/roo
sudo docker build -t roo .
