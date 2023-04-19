GOARCH ?= amd64
GOOS ?= linux
VERSION ?= latest
IMG ?= docker.io/desmo999r/formolcli:$(VERSION)
MANIFEST = formolcli-multiarch
BINDIR = ./bin
PLATFORMS ?= linux/arm64,linux/amd64

.PHONY: formolcli
formolcli: fmt vet
	GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINDIR)/formolcli main.go

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: docker-build-multiarch
docker-build-multiarch:
	buildah bud --manifest $(MANIFEST) --platform=$(PLATFORMS) .

.PHONY: docker-push
docker-push: 
	buildah manifest push --all --rm $(MANIFEST) "docker://$(IMG)"

.PHONY: docker-build-multiarch
docker-build-multiarch: 
	buildah bud --manifest $(MANIFEST) --platform linux/amd64,linux/arm64/v8 .

.PHONY: all
all: formolcli docker-build
