#!/bin/bash

while getopts v:t: option
do
case "${option}"
in
v) VERSION=${OPTARG};;
t) TAG=${OPTARG};;
esac
done

if [ -z "$VERSION" ]
then
	echo "Please provide a valid version with the -v flag. e.g: -v 1.0"
	exit 1
fi

if [ -z "$TAG" ]
then
	echo "Please provide a valid tag with the -t flag. e.g: -t stable (supported tags: dev, stable)"
	exit 1
fi

if [[ "$TAG" != "dev" && "$TAG" != "stable" ]]; then
	echo "Please use either dev or stable as tag"
	exit 1
fi

echo "This will upload a new mindfulbytes version to dockerhub"
echo "Version: $VERSION"
echo "Tag: $TAG"
echo ""

read -r -p "Are you sure? [y/N] " response
case "$response" in
    [yY][eE][sS]|[yY])
        docker buildx rm multibuilder
		
		docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
		
		docker buildx create --name multibuilder
		docker buildx use multibuilder
		
		if [[ "$TAG" == "stable" ]]; then	
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-api:$VERSION -f env/docker/Dockerfile.api . --push
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-api:latest -f env/docker/Dockerfile.api . --push
        
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-crawler:$VERSION -f env/docker/Dockerfile.crawler . --push
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-crawler:latest -f env/docker/Dockerfile.crawler . --push	
		
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-notifier:$VERSION -f env/docker/Dockerfile.notifier . --push
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-notifier:latest -f env/docker/Dockerfile.notifier . --push	
		fi

		if [[ "$TAG" == "dev" ]]; then
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-api:${VERSION}-dev -f env/docker/Dockerfile.api . --push
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-api:latest-dev -f env/docker/Dockerfile.api . --push
        
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-crawler:${VERSION}-dev -f env/docker/Dockerfile.crawler . --push
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-crawler:latest-dev -f env/docker/Dockerfile.crawler . --push
		
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-notifier:${VERSION}-dev -f env/docker/Dockerfile.notifier . --push
			docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t bbernhard/mindfulbytes-notifier:latest-dev -f env/docker/Dockerfile.notifier . --push
		fi

		;;
    *)
        echo "Aborting"
		exit 1
        ;;
esac
