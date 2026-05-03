.PHONY: build test lint install clean

build:
	go build -o bin/craigslist-pp-cli ./cmd/craigslist-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/craigslist-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/craigslist-pp-mcp ./cmd/craigslist-pp-mcp

install-mcp:
	go install ./cmd/craigslist-pp-mcp

build-all: build build-mcp
