.PHONY: build test lint install clean

build:
	go build -o bin/notion-pp-cli ./cmd/notion-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/notion-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/notion-pp-mcp ./cmd/notion-pp-mcp

install-mcp:
	go install ./cmd/notion-pp-mcp

build-all: build build-mcp
