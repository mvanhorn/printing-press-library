.PHONY: build test lint install clean

build:
	go build -o bin/goodreads-pp-cli ./cmd/goodreads-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/goodreads-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/goodreads-pp-mcp ./cmd/goodreads-pp-mcp

install-mcp:
	go install ./cmd/goodreads-pp-mcp

build-all: build build-mcp
