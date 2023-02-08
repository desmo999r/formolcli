GOARCH ?= amd64
GOOS ?= linux
IMG ?= docker.io/desmo999r/formolcli:latest
BINDIR = ./bin

.PHONY: formolcli
formolcli: fmt vet
	GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINDIR)/formolcli main.go

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: docker-build
docker-build: formolcli
	buildah bud --disable-compression --format=docker --platform $(GOOS)/$(GOARCH) --manifest $(IMG) Dockerfile.$(GOARCH)

.PHONY: docker-push
docker-push: docker-build
	buildah push $(IMG)

.PHONY: all
all: formolcli docker-build
