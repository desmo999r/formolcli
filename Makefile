IMG-deployment ?= desmo999r/formolcli:latest
docker-build-deployment:
	podman build --disable-compression --format=docker --file Dockerfile.deployment -t ${IMG-deployment}

docker-push-deployment:
	podman push ${IMG-deployment}

deployment: docker-build-deployment docker-push-deployment

all: deployment
