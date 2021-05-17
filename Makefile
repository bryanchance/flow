GOOS?=
GOARCH?=
COMMIT?=`git rev-parse --short HEAD`
APP=finca
DAEMON=finca
CLI=fctl
COMPOSITOR=finca-compositor
REPO?=git.underland.io/ehazlett/$(APP)
TAG?=dev
BUILD?=-d
VERSION?=dev
BUILD_ARGS?=
PACKAGES=$(shell go list ./... | grep -v -e /vendor/)
EXTENSIONS=$(wildcard extensions/*)
CYCLO_PACKAGES=$(shell go list ./... | grep -v /vendor/ | sed "s/github.com\/$(NAMESPACE)\/$(APP)\///g" | tail -n +2)
VAB_ARGS?=
CWD=$(PWD)
WORKER_IMAGE?=r.underland.io/apps/finca:latest

all: binaries

protos:
	@>&2 echo " -> building protobufs for grpc"
	@echo ${PACKAGES} | xargs protobuild -quiet

binaries: $(DAEMON) $(CLI) $(COMPOSITOR)

bindir:
	@mkdir -p bin

$(CLI): bindir
	@>&2 echo " -> building $(CLI) ${COMMIT}${BUILD}"
	@cd cmd/$(CLI) && CGO_ENABLED=0 go build -mod=mod -installsuffix cgo -ldflags "-w -X $(REPO)/version.GitCommit=$(COMMIT) -X $(REPO)/version.Version=$(VERSION) -X $(REPO)/version.Build=$(BUILD)" -o ../../bin/$(CLI) .

$(DAEMON): bindir
	@>&2 echo " -> building $(DAEMON) ${COMMIT}${BUILD}"
	@cd cmd/$(DAEMON) && CGO_ENABLED=0 go build -mod=mod -installsuffix cgo -ldflags "-w -X $(REPO)/version.GitCommit=$(COMMIT) -X $(REPO)/version.Version=$(VERSION) -X $(REPO)/version.Build=$(BUILD)" -o ../../bin/$(DAEMON) .

$(COMPOSITOR): bindir
	@>&2 echo " -> building $(COMPOSITOR) ${COMMIT}${BUILD}"
	@cd cmd/$(COMPOSITOR) && CGO_ENABLED=0 go build -mod=mod -installsuffix cgo -ldflags "-w -X $(REPO)/version.GitCommit=$(COMMIT) -X $(REPO)/version.Version=$(VERSION) -X $(REPO)/version.Build=$(BUILD)" -o ../../bin/$(COMPOSITOR) .

vet:
	@echo " -> $@"
	@test -z "$$(go vet ${PACKAGES} 2>&1 | tee /dev/stderr)"

worker:
	@cd worker; vab build -r ${WORKER_IMAGE} -p -c ../ .

lint:
	@echo " -> $@"
	@golint -set_exit_status ${PACKAGES}

cyclo:
	@echo " -> $@"
	@gocyclo -over 20 ${CYCLO_PACKAGES}

check: vet lint

test:
	@go test -mod=mod -short -v -cover $(TEST_ARGS) ${PACKAGES}

clean:
	@rm -rf bin/

.PHONY: protos clean docs check test install $(DAEMON) $(CLI) binaries worker
