#!/bin/bash

mkdir -p ./bin
echo "build loxi-ccm"
go build -o ./bin/loxi-cloud-controller-manager ./cmd