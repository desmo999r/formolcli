# Build a small image
FROM --platform=$BUILDPLATFORM golang:alpine3.17 AS builder

WORKDIR /go/src
COPY . .
ARG TARGETOS TARGETARCH
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o bin/formolcli main.go

FROM alpine:3.17
RUN apk add --no-cache su-exec restic postgresql-client
COPY --from=builder /go/src/bin/formolcli /usr/local/bin

# Command to run
ENTRYPOINT ["/usr/local/bin/formolcli"]
CMD ["--help"]
