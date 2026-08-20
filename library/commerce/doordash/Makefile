GO ?= /home/hermes/.local/bin/go

.PHONY: build test lint install clean

build:
	$(GO) build -o bin/doordash-pp-cli ./cmd/doordash-pp-cli

test:
	$(GO) test ./...

lint:
	golangci-lint run

install:
	$(GO) install ./cmd/doordash-pp-cli

clean:
	rm -rf bin/

build-mcp:
	$(GO) build -o bin/doordash-pp-mcp ./cmd/doordash-pp-mcp

install-mcp:
	$(GO) install ./cmd/doordash-pp-mcp

build-all: build build-mcp
