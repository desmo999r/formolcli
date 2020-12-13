IMG ?= desmo999r/formolcli:latest
docker-build:
	podman build --disable-compression --format=docker . -t ${IMG}

docker-push:
	podman push ${IMG}
