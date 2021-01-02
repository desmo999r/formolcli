# Build a small image
FROM arm32v7/alpine:3.12

RUN apk add --no-cache restic postgresql-client
COPY bin/formolcli /usr/local/bin

# Command to run
ENTRYPOINT ["/usr/local/bin/formolcli"]
CMD ["--help"]
