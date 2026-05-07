.PHONY: build test lint install clean

build:
	go build -o bin/wanderlust-goat-pp-cli ./cmd/wanderlust-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/wanderlust-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/wanderlust-goat-pp-mcp ./cmd/wanderlust-goat-pp-mcp

install-mcp:
	go install ./cmd/wanderlust-goat-pp-mcp

build-all: build build-mcp
