.PHONY: build test lint install clean

build:
	go build -o bin/ted-search-pp-cli ./cmd/ted-search-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ted-search-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ted-search-pp-mcp ./cmd/ted-search-pp-mcp

install-mcp:
	go install ./cmd/ted-search-pp-mcp

build-all: build build-mcp
