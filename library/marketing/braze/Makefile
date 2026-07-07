.PHONY: build test lint install clean

build:
	go build -o bin/braze-pp-cli ./cmd/braze-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/braze-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/braze-pp-mcp ./cmd/braze-pp-mcp

install-mcp:
	go install ./cmd/braze-pp-mcp

build-all: build build-mcp
