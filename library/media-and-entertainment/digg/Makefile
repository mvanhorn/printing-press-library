.PHONY: build test lint install clean

build:
	go build -o bin/digg-pp-cli ./cmd/digg-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/digg-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/digg-pp-mcp ./cmd/digg-pp-mcp

install-mcp:
	go install ./cmd/digg-pp-mcp

build-all: build build-mcp
