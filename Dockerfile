# Build a small image
FROM --platform=${BUILDPLATFORM} golang:alpine3 AS builder
ARG TARGETOS
ARG TARGETARCH 
ARG TARGETPLATFORM

WORKDIR /go/src
COPY go.mod go.mod
COPY go.sum go.sum
COPY formol/ formol/
RUN go mod download
COPY main.go main.go
COPY cmd/ cmd/
COPY standalone/ standalone/
COPY controllers/ controllers/
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o bin/formolcli main.go

FROM --platform=${TARGETPLATFORM} alpine:3
RUN apk add --no-cache su-exec restic
COPY --from=builder /go/src/bin/formolcli /usr/local/bin

# Command to run
ENTRYPOINT ["/usr/local/bin/formolcli"]
CMD ["--help"]
