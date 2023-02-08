GOARCH ?= amd64
GOOS ?= linux
IMG ?= desmo999r/formolcli:latest
BINDIR = ./bin

$(BINDIR)/formolcli: fmt vet
	GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINDIR)/formolcli main.go

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: docker-build
docker-build: $(BINDIR)/formolcli
	buildah build --disable-compression --format=docker --platform $(GOOS)/$(GOARCH) -t $(IMG) .

.PHONY: docker-push
docker-push: docker-build
	buildah push $(IMG)

.PHONY: all
all: $(BINDIR)/formolcli docker-build
