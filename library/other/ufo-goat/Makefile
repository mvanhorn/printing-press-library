.PHONY: build test lint install clean

build:
	go build -o bin/ufo-goat-pp-cli ./cmd/ufo-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ufo-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ufo-goat-pp-mcp ./cmd/ufo-goat-pp-mcp

install-mcp:
	go install ./cmd/ufo-goat-pp-mcp

build-all: build build-mcp
