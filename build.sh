#!/bin/bash
go get github.com/sfproductlabs/roo
go install github.com/sfproductlabs/roo
go build
sudo docker build -t roo .
