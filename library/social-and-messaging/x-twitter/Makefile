.PHONY: build test lint install clean

build:
	go build -o bin/x-twitter-pp-cli ./cmd/x-twitter-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/x-twitter-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/x-twitter-pp-mcp ./cmd/x-twitter-pp-mcp

install-mcp:
	go install ./cmd/x-twitter-pp-mcp

build-all: build build-mcp
