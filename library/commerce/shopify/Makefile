.PHONY: build test lint install clean

build:
	go build -o bin/shopify-pp-cli ./cmd/shopify-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/shopify-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/shopify-pp-mcp ./cmd/shopify-pp-mcp

install-mcp:
	go install ./cmd/shopify-pp-mcp

build-all: build build-mcp
