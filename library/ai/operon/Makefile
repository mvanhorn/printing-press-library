.PHONY: build test lint install clean

build:
	go build -o bin/operon-pp-cli ./cmd/operon-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/operon-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/operon-pp-mcp ./cmd/operon-pp-mcp

install-mcp:
	go install ./cmd/operon-pp-mcp

build-all: build build-mcp
