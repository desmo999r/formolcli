GOARCH ?= amd64

.PHONY: formolcli
formolcli: fmt vet
	GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -o bin/formolcli main.go

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: all
all: formolcli
