.PHONY: build test lint install clean

build:
	go build -o bin/indeed-pp-cli ./cmd/indeed-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/indeed-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/indeed-pp-mcp ./cmd/indeed-pp-mcp

install-mcp:
	go install ./cmd/indeed-pp-mcp

build-all: build build-mcp
