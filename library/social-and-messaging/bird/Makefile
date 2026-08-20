.PHONY: build test lint install clean

build:
	go build -o bin/bird-pp-cli ./cmd/bird-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bird-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bird-pp-mcp ./cmd/bird-pp-mcp

install-mcp:
	go install ./cmd/bird-pp-mcp

build-all: build build-mcp
