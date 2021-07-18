# Build a small image
FROM arm64v8/alpine:3.14

RUN apk add --no-cache su-exec restic postgresql-client
COPY bin/formolcli /usr/local/bin

# Command to run
ENTRYPOINT ["/usr/local/bin/formolcli"]
CMD ["--help"]
