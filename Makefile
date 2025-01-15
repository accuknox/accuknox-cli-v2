# SPDX-License-Identifier: Apache-2.0
# Copyright 2022 Authors of KubeArmor

CURDIR     := $(shell pwd)
INSTALLDIR := $(shell go env GOPATH)/bin/

ifeq (, $(shell which govvv))
$(shell go install github.com/ahmetb/govvv@latest)
endif

PKG      := $(shell go list ./pkg/version)
GIT_INFO := $(shell govvv -flags -pkg $(PKG))

.PHONY: build
build:
	cd $(CURDIR); go mod tidy; CGO_ENABLED=0 go build -ldflags "-w -s ${GIT_INFO}" -o knoxctl

.PHONY: debug
debug:
	cd $(CURDIR); go mod tidy; CGO_ENABLED=0 go build -ldflags "${GIT_INFO}" -o knoxctl

.PHONY: install
install: build
	install -m 0755 knoxctl $(DESTDIR)$(INSTALLDIR)

.PHONY: clean
clean:
	cd $(CURDIR); rm -f knoxctl


.PHONY: protobuf
vm-protobuf:
	cd $(CURDIR)/vm/protobuf; protoc --proto_path=. --go_opt=paths=source_relative --go_out=plugins=grpc:. vm.proto

.PHONY: gofmt
gofmt:
	cd $(CURDIR); gofmt -s -d $(shell find . -type f -name '*.go' -print)
	cd $(CURDIR); test -z "$(shell gofmt -s -l $(shell find . -type f -name '*.go' -print) | tee /dev/stderr)"

.PHONY: golint
golint:
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
gosec:
ifeq (, $(shell which gosec))
	@{ \
	set -e ;\
	curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b  ~/.bin ;\
	}
endif
	cd $(CURDIR); ~/.bin/gosec ./...

.PHONY: test 
test: 
	./scripts/tests.sh

.PHONY: local-release
local-release: build
ifeq (, $(shell which goreleaser))
	@{ \
	set -e ;\
	go install github.com/goreleaser/goreleaser@latest ;\
	}
endif
	cd $(CURDIR); VERSION=$(shell git describe --tags --always --dirty) goreleaser release --clean --skip=publish --skip=sign --skip=validate --snapshot
