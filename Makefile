#!/usr/bin/make -f

VERSION = $(shell git rev-parse HEAD)

.PHONY: help
help:
	@echo
	@echo "Commands"
	@echo "========"
	@echo
	@sed -n '/^[a-zA-Z0-9_-]*:/s/:.*//p' < Makefile | grep -v -E 'default|help.*' | sort


.PHONY: build
build:
	CGO_ENABLED=1 go build ./...


.PHONY: test
test: test-unit

.PHONY: test-unit
test-unit:
	@VERSION=$(VERSION) go test -mod=readonly ./...

.PHONY: test-race
test-race:
	@VERSION=$(VERSION) go test -mod=readonly --race ./...


.PHONY: build-linux-static
build-linux-static:
	$(MAKE) -C examples/wasmd build-linux-static


###############################################################################
###                                Linting                                  ###
###############################################################################

.PHONY: format-tools
format-tools:
	go install mvdan.cc/gofumpt@v0.3.1
	go install github.com/client9/misspell/cmd/misspell@v0.3.4

.PHONY: lint
lint: format-tools
	golangci-lint run --tests=false
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" | xargs gofumpt -d -s

.PHONY: format
format: format-tools
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" -not -path "./client/lcd/statik/statik.go" | xargs gofumpt -w -s
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" -not -path "./client/lcd/statik/statik.go" | xargs misspell -w
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" -not -path "./client/lcd/statik/statik.go" | xargs goimports -w -local github.com/alpe/cosmos-tracing
