.PHONY: build test lint install clean

build:
	go build -o bin/monarch-pp-cli ./cmd/monarch-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/monarch-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/monarch-pp-mcp ./cmd/monarch-pp-mcp

install-mcp:
	go install ./cmd/monarch-pp-mcp

build-all: build build-mcp
