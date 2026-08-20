.PHONY: build test lint install clean

build:
	go build -o bin/arxiv-pp-cli ./cmd/arxiv-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/arxiv-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/arxiv-pp-mcp ./cmd/arxiv-pp-mcp

install-mcp:
	go install ./cmd/arxiv-pp-mcp

build-all: build build-mcp
