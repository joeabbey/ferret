GOPATH?=$(shell go env GOPATH)
export PATH := $(GOPATH)/bin:$(PATH)

NOW := $(shell date -u +%Y-%m-%dT%H:%MZ)
GITCOMMIT?=$(shell git describe --always)
VERSION?=$(NOW)-$(GITCOMMIT)-dev

PKG_LIST = $(shell go list ./... | grep -v /vendor/)

all: install

#Fix termui's brokeness
.PHONY: help
help: ## print this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

install: ## install into $GOPATH/bin
	go install -v

.PHONY: vet
vet: ## vet sources
	go vet $(PKG_LIST)

.PHONY: lint
lint: $(GOPATH)/bin/golint ## lint sources
	golint -set_exit_status -min_confidence=0.4 $(PKG_LIST)

.PHONY: doc
doc: ## run godoc server on http://localhost:6060/pkg
	godoc -http=":6060"


$(GOPATH)/bin/golint:
	go install golang.org/x/lint/golint@latest

