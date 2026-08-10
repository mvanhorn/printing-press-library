.PHONY: build test lint install clean

build:
	go build -o bin/monday-pp-cli ./cmd/monday-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/monday-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/monday-pp-mcp ./cmd/monday-pp-mcp

install-mcp:
	go install ./cmd/monday-pp-mcp

build-all: build build-mcp
