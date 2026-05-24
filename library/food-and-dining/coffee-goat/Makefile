.PHONY: build test lint install clean

build:
	go build -o bin/coffee-goat-pp-cli ./cmd/coffee-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/coffee-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/coffee-goat-pp-mcp ./cmd/coffee-goat-pp-mcp

install-mcp:
	go install ./cmd/coffee-goat-pp-mcp

build-all: build build-mcp
