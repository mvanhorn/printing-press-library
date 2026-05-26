.PHONY: build test lint install clean

build:
	go build -o bin/obsidian-pp-cli ./cmd/obsidian-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/obsidian-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/obsidian-pp-mcp ./cmd/obsidian-pp-mcp

install-mcp:
	go install ./cmd/obsidian-pp-mcp

build-all: build build-mcp
