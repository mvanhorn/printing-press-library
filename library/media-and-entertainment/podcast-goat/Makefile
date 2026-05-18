.PHONY: build test lint install clean

build:
	go build -o bin/podcast-goat-pp-cli ./cmd/podcast-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/podcast-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/podcast-goat-pp-mcp ./cmd/podcast-goat-pp-mcp

install-mcp:
	go install ./cmd/podcast-goat-pp-mcp

build-all: build build-mcp
