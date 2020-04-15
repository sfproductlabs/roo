#!/bin/bash
rm -rf cluster-data
cat ~/.DH_TOKEN | sudo docker login --username sfproductlabs --password-stdin
sudo docker tag $(sudo docker images -q | head -1) sfproductlabs/roo:latest
sudo docker push sfproductlabs/roo:latest