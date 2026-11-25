# SPDX-License-Identifier: Apache-2.0
# Copyright 2022 Authors of KubeArmor
CURDIR     := $(shell pwd)
# Trivy v0.72.0 uses the Go 1.26 jsonv2 API removed in Go 1.27.
GOTOOLCHAIN ?= go1.26.5
export GOTOOLCHAIN
INSTALLDIR := $(shell go env GOPATH)/bin/
GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
export GOEXPERIMENT=jsonv2

# Required so Go bypasses the public module proxy for private accuknox packages.
export GOPRIVATE = github.com/accuknox/*

# On local dev machines (CI sets CI=true automatically), rewrite HTTPS GitHub
# URLs to SSH so private repos are fetched using SSH keys instead of prompting
# for a username/password. CI workflows configure HTTPS token auth separately
# via a global git config step, so we skip this there.
ifndef CI
export GIT_CONFIG_COUNT = 1
export GIT_CONFIG_KEY_0 = url.git@github.com:.insteadOf
export GIT_CONFIG_VALUE_0 = https://github.com/
endif

# Compile RRA submodule beforehand for embedding in Knoxctl
RRADIR      := $(CURDIR)/pkg/vm
# Compile cbomkit-theia submodule beforehand for embedding in Knoxctl
CBOMDIR     := $(CURDIR)/pkg/cbom
# Compile trivy submodule beforehand for embedding in Knoxctl (as imgscan)
IMGSCANDIR  := $(CURDIR)/pkg/imagescan

# Resolve the imgscan binary name (imgscan.exe on Windows, imgscan elsewhere)
ifeq ($(GOOS),windows)
IMGSCAN_BIN := imgscan.exe
else
IMGSCAN_BIN := imgscan
endif

.DEFAULT_GOAL := build

prebuild:
	@if git submodule status --recursive | grep -Eq '^[-+U]'; then git submodule update --init --recursive; fi
	cd $(RRADIR)/RRA; go mod tidy; CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-w -s ${GIT_INFO}" -o $(RRADIR)/rra-agent
	cd $(CBOMDIR)/cbomkit-theia; go mod tidy; CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-w -s" -o $(CBOMDIR)/cbomkit-theia-bin
	cd $(IMGSCANDIR)/trivy; go mod tidy; CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-w -s" -o $(CURDIR)/pkg/tools/bins/$(IMGSCAN_BIN) ./cmd/trivy/
	touch $(CURDIR)/pkg/tools/bins/placeholder

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
	cd $(CURDIR); rm -fr knoxctl dist


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
gosec:prebuild
ifeq (, $(shell which gosec))
	@{ \
	set -e ;\
	GOSEC_TMP_DIR=$$(mktemp -d) ;\
	cd $$GOSEC_TMP_DIR ;\
	go mod init tmp ;\
	go get github.com/securego/gosec/v2/cmd/gosec ;\
	go install github.com/securego/gosec/v2/cmd/gosec ;\
	rm -rf $$GOSEC_TMP_DIR ;\
	}
endif
	cd $(CURDIR);gosec -exclude-dir=pkg/vm/RRA -exclude-dir=pkg/cbom/cbomkit-theia -exclude-dir=pkg/imagescan/trivy ./...

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
	cd $(CURDIR); VERSION=$(shell git describe --tags --always --dirty) goreleaser release --clean --skip=publish --skip=sign --skip=validate --snapshot --parallelism 1
