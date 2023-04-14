#!/bin/bash
oldver=`perl -nle'print for /Roo\.\ Version\ (.*)\".*$/g' ./roo/roo.go`
newver=`expr $oldver + 1`
sed -i -- "s/Roo\.\ Version\ $oldver/Roo\.\ Version\ $newver/g" "./roo/roo.go"
bash -c "rm roo/roo.go-- || exit 0"
sudo rm -rf cluster-data*
sudo rm -rf roo/cluster-data*
sudo docker build -t roo .
#IF THE ABOVE FAILS
#error getting credentials - err: exit status 1
# just remove
#"credsStore": "secretservice" line from the docker config file
#in ~/.docker/config.json
#test with 
#docker run --rm -it roo
