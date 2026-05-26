.PHONY: build test lint install clean

build:
	go build -o bin/home-goat-pp-cli ./cmd/home-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/home-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/home-goat-pp-mcp ./cmd/home-goat-pp-mcp

install-mcp:
	go install ./cmd/home-goat-pp-mcp

build-all: build build-mcp
