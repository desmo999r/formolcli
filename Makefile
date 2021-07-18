.PHONY: all formolcli docker docker-build docker-push

IMG ?= desmo999r/formolcli:latest

formolcli: fmt vet
	GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/formolcli main.go

test: fmt vet
	go test ./... -coverprofile cover.out

fmt:
	go fmt ./...

vet:
	go vet ./...

docker-build:
	buildah bud --disable-compression --format=docker -t ${IMG} .

docker-push:
	buildah push ${IMG}

docker: formolcli docker-build docker-push

all: docker
