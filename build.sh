#!/bin/bash
go get github.com/sfproductlabs/roo
#go install github.com/sfproductlabs/roo
str=`egrep "Roo. Version [0-9]+" ./roo/roo.go`
oldver=$(echo $str | tr -d "Roo. Version ")
newver=`expr $oldver + 1`
sed -i "" -E "s/Roo. Version $oldver\$/Roo. Version $newver/g" "./roo/roo.go"
CGO_LDFLAGS="-lrocksdb" go build -v -o rood github.com/sfproductlabs/roo/v3/roo
sudo docker build -t roo .
