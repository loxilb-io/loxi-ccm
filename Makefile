.DEFAULT_GOAL := build
bin=loxi-cloud-controller-manager
tag?=latest

build:
	@mkdir -p ./bin
	go build -o ./bin/${bin} ./cmd

clean:
	go clean ./cmd

docker: 
	docker build -t ghcr.io/loxilb-io/loxi-ccm:${tag} .
