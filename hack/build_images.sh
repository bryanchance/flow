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
STELLAR_ARGS=${STELLAR_ARGS:-""}

if [ ! -z "${PUSH}" ] && [ "${PUSH}" != "n" ] && [ "${PUSH}" != "N" ]; then
    PUSH="--push"
fi

if [ ! -z "${UPDATE_LATEST}" ] && [ "${UPDATE_LATEST}" != "n" ] && [ "${UPDATE_LATEST}" != "N" ]; then
    DAEMON_EXTRA="-t ${REGISTRY}/${DAEMON}:latest"
fi

# check for skip
if [ -z "$SKIP_FLOW" ]; then
    echo " -> building $DAEMON"
    stellar ${STELLAR_ARGS} apps build -a VERSION=${VERSION} -a BUILD=${BUILD} -a COMMIT=${COMMIT} -a WORKFLOW="${workflow}" -t ${REGISTRY}/${workflow}:${TAG} ${IMAGE_BUILD_EXTRA} ${DAEMON_EXTRA} ${PUSH} .
fi

if [ -z "$SKIP_WORKFLOWS" ]; then
    for workflow in $WORKFLOWS; do
        pushd cmd/${workflow}
        echo " -> building $workflow"
        if [ ! -z "${UPDATE_LATEST}" ] && [ "${UPDATE_LATEST}" != "n" ] && [ "${UPDATE_LATEST}" != "N" ]; then
            WORKFLOW_EXTRA="-t ${REGISTRY}/${workflow}:latest"
        fi
        stellar ${STELLAR_ARGS} apps build --build-context ../../ -a VERSION=${VERSION} -a BUILD=${BUILD} -a COMMIT=${COMMIT} -a PROCESSOR="${workflow}" -t ${REGISTRY}/${workflow}:${TAG} ${IMAGE_BUILD_EXTRA} ${WORKFLOW_EXTRA} ${PUSH} .
        popd
    done
fi
