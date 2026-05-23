GO ?= /home/hermes/.local/bin/go

.PHONY: build test lint install clean

build:
	$(GO) build -o bin/doordash-consumer-pp-cli ./cmd/doordash-consumer-pp-cli

test:
	$(GO) test ./...

lint:
	golangci-lint run

install:
	$(GO) install ./cmd/doordash-consumer-pp-cli

clean:
	rm -rf bin/

build-mcp:
	$(GO) build -o bin/doordash-consumer-pp-mcp ./cmd/doordash-consumer-pp-mcp

install-mcp:
	$(GO) install ./cmd/doordash-consumer-pp-mcp

build-all: build build-mcp
