.PHONY: build test lint install clean

build:
	go build -o bin/apple-docs-pp-cli ./cmd/apple-docs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/apple-docs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/apple-docs-pp-mcp ./cmd/apple-docs-pp-mcp

install-mcp:
	go install ./cmd/apple-docs-pp-mcp

build-all: build build-mcp
