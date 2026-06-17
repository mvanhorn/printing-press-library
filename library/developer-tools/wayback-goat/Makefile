.PHONY: build test lint install clean

build:
	go build -o bin/wayback-goat-pp-cli ./cmd/wayback-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/wayback-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/wayback-goat-pp-mcp ./cmd/wayback-goat-pp-mcp

install-mcp:
	go install ./cmd/wayback-goat-pp-mcp

build-all: build build-mcp
