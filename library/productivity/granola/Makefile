.PHONY: build test lint install clean

build:
	go build -o bin/granola-pp-cli ./cmd/granola-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/granola-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/granola-pp-mcp ./cmd/granola-pp-mcp

install-mcp:
	go install ./cmd/granola-pp-mcp

build-all: build build-mcp
