.PHONY: build test lint install clean

build:
	go build -o bin/surgegraph-pp-cli ./cmd/surgegraph-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/surgegraph-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/surgegraph-pp-mcp ./cmd/surgegraph-pp-mcp

install-mcp:
	go install ./cmd/surgegraph-pp-mcp

build-all: build build-mcp
