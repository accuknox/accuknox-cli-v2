# SPDX-License-Identifier: Apache-2.0
# Copyright 2022 Authors of KubeArmor
CURDIR     := $(shell pwd)
INSTALLDIR := $(shell go env GOPATH)/bin/

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Compile RRA submodule beforehand for embeding in Knoxctl
RRADIR := $(CURDIR)/pkg/vm
prebuild:
	git submodule update --init --recursive
	cd $(RRADIR)/RRA; go mod tidy; CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-w -s ${GIT_INFO}" -o $(RRADIR)/rra

ifeq (, $(shell which govvv))
$(shell go install github.com/ahmetb/govvv@latest)
endif

PKG      := $(shell go list ./pkg/version)
GIT_INFO := $(shell govvv -flags -pkg $(PKG))

.PHONY: build
build: prebuild

	cd $(CURDIR); go mod tidy; CGO_ENABLED=0 go build -ldflags "-w -s ${GIT_INFO}" -o knoxctl

.PHONY: debug
debug: prebuild
	cd $(CURDIR); go mod tidy; CGO_ENABLED=0 go build -ldflags "${GIT_INFO}" -o knoxctl

.PHONY: install
install: prebuild build
	install -m 0755 knoxctl $(DESTDIR)$(INSTALLDIR)

.PHONY: clean
clean:
	cd $(CURDIR); rm -f knoxctl


.PHONY: protobuf
vm-protobuf:
	cd $(CURDIR)/vm/protobuf; protoc --proto_path=. --go_opt=paths=source_relative --go_out=plugins=grpc:. vm.proto

.PHONY: gofmt
gofmt: prebuild
	cd $(CURDIR); gofmt -s -d $(shell find . -type f -name '*.go' -print)
	cd $(CURDIR); test -z "$(shell gofmt -s -l $(shell find . -type f -name '*.go' -print) | tee /dev/stderr)"

.PHONY: golint
golint: prebuild
ifeq (, $(shell which golint))
	@{ \
	set -e ;\
	GOLINT_TMP_DIR=$$(mktemp -d) ;\
	cd $$GOLINT_TMP_DIR ;\
	go mod init tmp ;\
	go get -u golang.org/x/lint/golint ;\
	rm -rf $$GOLINT_TMP_DIR ;\
	}
endif
	cd $(CURDIR); golint ./...

.PHONY: gosec
gosec: prebuild
ifeq (, $(shell which gosec))
	@{ \
	set -e ;\
	curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b  ~/.bin ;\
	}
endif
	cd $(CURDIR); ~/.bin/gosec -exclude-dir=pkg/vm/RRA ./... 

.PHONY: test 
test: prebuild
	./scripts/tests.sh

.PHONY: local-release
local-release: prebuild build
ifeq (, $(shell which goreleaser))
	@{ \
	set -e ;\
	go install github.com/goreleaser/goreleaser@latest ;\
	}
endif
	cd $(CURDIR); VERSION=$(shell git describe --tags --always --dirty) goreleaser release --clean --skip=publish --skip=sign --skip=validate --snapshot
