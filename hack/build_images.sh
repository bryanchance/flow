#!/bin/bash
VERSION=${VERSION:-"dev"}
BUILD=${BUILD:-""}
REGISTRY=${REGISTRY:-"docker.io/ehazlett"}
TAG=${TAG:-"dev"}
DAEMON=flow
WORKFLOWS=$(ls cmd/ | grep flow-workflow)
IMAGE_BUILD_EXTRA="${IMAGE_BUILD_EXTRA:-""}"
PUSH=${PUSH:-""}

if [ ! -z "${PUSH}" ] && [ "${PUSH}" != "n" ] && [ "${PUSH}" != "N" ]; then
    PUSH="--push"
fi

echo " -> building ${VERSION}${BUILD}"

echo " -> building $DAEMON"
docker buildx build --build-arg VERSION=${VERSION} --build-arg BUILD=${BUILD} ${IMAGE_BUILD_EXTRA} -t ${REGISTRY}/${DAEMON}:${TAG} ${PUSH} -f Dockerfile .

for workflow in $WORKFLOWS; do
    echo " -> building $workflow"
    docker buildx build --build-arg VERSION=${VERSION} --build-arg BUILD=${BUILD} --build-arg WORKFLOW="${workflow}" ${IMAGE_BUILD_EXTRA} -t ${REGISTRY}/${workflow}:${TAG} ${PUSH} -f cmd/$workflow/Dockerfile .
done
