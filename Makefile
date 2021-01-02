.PHONY: all formolcli docker docker-build docker-push

IMG ?= desmo999r/formolcli:latest

formolcli: fmt vet
	GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o bin/formolcli main.go

fmt:
	go fmt ./...

vet:
	go vet ./...

docker-build:
	podman build --disable-compression --format=docker -t ${IMG} .

docker-push:
	podman push ${IMG}

docker: formolcli docker-build docker-push

all: docker
