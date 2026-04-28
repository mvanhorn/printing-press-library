.PHONY: build test lint install clean

build:
	go build -o bin/twitter-pp-cli ./cmd/twitter-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/twitter-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/twitter-pp-mcp ./cmd/twitter-pp-mcp

install-mcp:
	go install ./cmd/twitter-pp-mcp

build-all: build build-mcp
