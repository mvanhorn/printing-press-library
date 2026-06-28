.PHONY: build test lint install clean

build:
	go build -o bin/apple-search-ads-pp-cli ./cmd/apple-search-ads-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/apple-search-ads-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/apple-search-ads-pp-mcp ./cmd/apple-search-ads-pp-mcp

install-mcp:
	go install ./cmd/apple-search-ads-pp-mcp

build-all: build build-mcp
