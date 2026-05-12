.PHONY: build test lint install clean

build:
	go build -o bin/tavily-pp-cli ./cmd/tavily-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tavily-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tavily-pp-mcp ./cmd/tavily-pp-mcp

install-mcp:
	go install ./cmd/tavily-pp-mcp

build-all: build build-mcp
