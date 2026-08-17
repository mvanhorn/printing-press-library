.PHONY: build test lint install clean

build:
	go build -o bin/npm-pp-cli ./cmd/npm-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/npm-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/npm-pp-mcp ./cmd/npm-pp-mcp

install-mcp:
	go install ./cmd/npm-pp-mcp

build-all: build build-mcp
