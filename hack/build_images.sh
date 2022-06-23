#!/bin/bash
VERSION=${VERSION:-"dev"}
BUILD=${BUILD:-"-d"}
REGISTRY=${REGISTRY:-"docker.io/ehazlett"}
TAG=${TAG:-"dev"}
DAEMON=flow
WORKER=flow-worker
WORKFLOWS=$(ls cmd/ | grep flow-workflow)
IMAGE_BUILD_EXTRA=""
PUSH=${PUSH:-""}

if [ ! -z "${PUSH}" ] && [ "${PUSH}" != "n" ] && [ "${PUSH}" != "N" ]; then
    PUSH="--push"
fi

echo " -> building $DAEMON"
docker buildx build --build-arg VERSION=$VERSION --build-arg BUILD=$BUILD ${IMAGE_BUILD_EXTRA} -t ${REGISTRY}/${DAEMON}:${TAG} ${PUSH} -f Dockerfile .

echo " -> building $WORKER"
docker buildx build --build-arg VERSION=$VERSION --build-arg BUILD=$BUILD ${IMAGE_BUILD_EXTRA} -t ${REGISTRY}/${WORKER}:${TAG} ${PUSH} -f Dockerfile.worker .

for workflow in $WORKFLOWS; do
    echo " -> building $workflow"
    docker buildx build --build-arg VERSION=$VERSION --build-arg BUILD=$BUILD --build-arg WORKFLOW="${workflow}" ${IMAGE_BUILD_EXTRA} -t ${REGISTRY}/${workflow}:${TAG} ${PUSH} -f cmd/$workflow/Dockerfile .
done

