#!/bin/bash

tag="0.1"

echo "building loxi-ccm docker image..."
sudo docker build -t eyes852/loxi-ccm:$tag -f ./Dockerfile .
echo "uploading loxi-ccm docker image to docker hub..."
sudo docker push eyes852/loxi-ccm:$tag