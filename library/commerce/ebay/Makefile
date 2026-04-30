.PHONY: build test lint install clean

build:
	go build -o bin/ebay-pp-cli ./cmd/ebay-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ebay-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ebay-pp-mcp ./cmd/ebay-pp-mcp

install-mcp:
	go install ./cmd/ebay-pp-mcp

build-all: build build-mcp
