.PHONY: build test lint install clean

build:
	go build -o bin/google-search-console-pp-cli ./cmd/google-search-console-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-search-console-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/google-search-console-pp-mcp ./cmd/google-search-console-pp-mcp

install-mcp:
	go install ./cmd/google-search-console-pp-mcp

build-all: build build-mcp
