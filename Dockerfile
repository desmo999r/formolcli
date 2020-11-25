FROM arm32v7/golang:alpine AS builder

# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=arm

# Move to working directory /build
WORKDIR /build

# Copy and download dependency using go mod
COPY src/go.mod .
COPY src/go.sum .
RUN go mod download

# Copy the code into the container
COPY src/ .

# Build the application
RUN go build -o formolcli .

# Move to /dist directory as the place for resulting binary folder
WORKDIR /dist

# Copy binary from build to main folder
RUN cp /build/formolcli .

# Build a small image
FROM scratch

COPY --from=builder /dist/formolcli /

# Command to run
ENTRYPOINT ["/formolcli"]
CMD ["--help"]
