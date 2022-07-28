#!/bin/bash
COMMIT=${COMMIT:-`git rev-parse --short HEAD`}
VERSION=${VERSION:-"dev"}
BUILD=${BUILD:-""}
REGISTRY=${REGISTRY:-"docker.io/ehazlett"}
TAG=${TAG:-"dev"}
DAEMON=flow
WORKFLOWS=${WORKFLOWS:-$(ls cmd/ | grep flow-workflow)}
IMAGE_BUILD_EXTRA="${IMAGE_BUILD_EXTRA:-""}"
PUSH=${PUSH:-""}
UPDATE_LATEST=${UPDATE_LATEST:-""}

if [ ! -z "${PUSH}" ] && [ "${PUSH}" != "n" ] && [ "${PUSH}" != "N" ]; then
    PUSH="--push"
fi

if [ ! -z "${UPDATE_LATEST}" ] && [ "${UPDATE_LATEST}" != "n" ] && [ "${UPDATE_LATEST}" != "N" ]; then
    DAEMON_EXTRA="-t ${REGISTRY}/${DAEMON}:latest"
fi

# check for skip
if [ -z "$SKIP_FLOW" ]; then
    echo " -> building $DAEMON"
    docker buildx build --build-arg VERSION=${VERSION} --build-arg COMMIT=${COMMIT} --build-arg BUILD=${BUILD} ${IMAGE_BUILD_EXTRA} ${DAEMON_EXTRA} -t ${REGISTRY}/${DAEMON}:${TAG} ${PUSH} -f Dockerfile .
fi

if [ -z "$SKIP_WORKFLOWS" ]; then
    for workflow in $WORKFLOWS; do
        echo " -> building $workflow"
        if [ ! -z "${UPDATE_LATEST}" ] && [ "${UPDATE_LATEST}" != "n" ] && [ "${UPDATE_LATEST}" != "N" ]; then
            WORKFLOW_EXTRA="-t ${REGISTRY}/${workflow}:latest"
        fi
        docker buildx build --build-arg VERSION=${VERSION} --build-arg BUILD=${BUILD} --build-arg COMMIT=${COMMIT} --build-arg PROCESSOR="${workflow}" ${IMAGE_BUILD_EXTRA} -t ${REGISTRY}/${workflow}:${TAG} ${WORKFLOW_EXTRA} ${PUSH} -f cmd/$workflow/Dockerfile .
    done
fi
