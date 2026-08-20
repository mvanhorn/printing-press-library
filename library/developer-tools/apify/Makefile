.PHONY: build test lint install clean

build:
	go build -o bin/apify-pp-cli ./cmd/apify-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/apify-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/apify-pp-mcp ./cmd/apify-pp-mcp

install-mcp:
	go install ./cmd/apify-pp-mcp

build-all: build build-mcp
