#!/bin/bash
CGO_LDFLAGS="-lrocksdb"  go build -v -o rood github.com/sfproductlabs/roo/v3/roo && ./rood ./roo/config.json
