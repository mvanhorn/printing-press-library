.PHONY: build test lint install clean

build:
	go build -o bin/art-goat-pp-cli ./cmd/art-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/art-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/art-goat-pp-mcp ./cmd/art-goat-pp-mcp

install-mcp:
	go install ./cmd/art-goat-pp-mcp

build-all: build build-mcp
