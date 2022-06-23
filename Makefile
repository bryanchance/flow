GOOS?=$(shell go env GOOS)
GOARCH?=$(shell go env GOARCH)
COMMIT?=`git rev-parse --short HEAD`
REGISTRY?=docker.io/ehazlett
APP=flow
DAEMON=flow
CGO_ENABLED=0
CLI=fctl
REPO?=github.com/ehazlett/$(APP)
TAG?=dev
BUILD?=-d
VERSION?=dev
BUILD_ARGS?=
PACKAGES=$(shell go list ./... | grep -v -e /vendor/)
WORKFLOWS=$(wildcard cmd/flow-workflow-*)
CYCLO_PACKAGES=$(shell go list ./... | grep -v /vendor/ | sed "s/github.com\/$(NAMESPACE)\/$(APP)\///g" | tail -n +2)
VAB_ARGS?=
CWD=$(PWD)

ifeq ($(GOOS), windows)
	EXT=.exe
endif

all: binaries

protos:
	@>&2 echo " -> building protobufs for grpc"
	@echo ${PACKAGES} | xargs protobuild -quiet

binaries: $(DAEMON) $(CLI) $(WORKFLOWS)

workflows: $(WORKFLOWS)

daemon: $(DAEMON)

cli: $(CLI)

bindir:
	@mkdir -p bin

$(CLI): bindir
	@>&2 echo " -> building $(CLI) ${COMMIT}${BUILD} (${GOARCH})"
	@cd cmd/$(CLI) && CGO_ENABLED=0 go build -mod=mod -installsuffix cgo -ldflags "-w -X $(REPO)/version.GitCommit=$(COMMIT) -X $(REPO)/version.Version=$(VERSION) -X $(REPO)/version.Build=$(BUILD)" -o ../../bin/$(CLI)$(EXT) .

$(DAEMON): bindir
	@>&2 echo " -> building $(DAEMON) ${COMMIT}${BUILD} (${GOARCH})"
	@if [ "$(GOOS)" = "windows" ]; then echo "ERR: Flow server not supported on windows"; exit; fi; cd cmd/$(DAEMON) && CGO_ENABLED=0 go build -mod=mod -installsuffix cgo -ldflags "-w -X $(REPO)/version.GitCommit=$(COMMIT) -X $(REPO)/version.Version=$(VERSION) -X $(REPO)/version.Build=$(BUILD)" -o ../../bin/$(DAEMON)$(EXT) .

$(WORKFLOWS): bindir
	@echo " -> building $(shell basename $@) ${COMMIT}${BUILD} (${GOARCH})"
	@cd "$@" && CGO_ENABLED=0 go build -mod=mod -installsuffix cgo -ldflags "-w -X $(REPO)/version.GitCommit=$(COMMIT) -X $(REPO)/version.Version=$(VERSION) -X $(REPO)/version.Build=$(BUILD)" -o ../../bin/$(basename $$@)$(EXT) .

vet:
	@echo " -> $@"
	@test -z "$$(go vet ${PACKAGES} 2>&1 | tee /dev/stderr)"

lint:
	@echo " -> $@"
	@golint -set_exit_status ${PACKAGES}

check: vet lint

test:
	@go test -mod=mod -short -v -cover $(TEST_ARGS) ${PACKAGES}

clean:
	@rm -rf bin/

.PHONY: protos clean docs check test install $(DAEMON) $(CLI) $(WORKFLOWS) binaries
