.PHONY: build test lint install clean

build:
	go build -o bin/claude-agent-sdk-python-docs-pp-cli ./cmd/claude-agent-sdk-python-docs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/claude-agent-sdk-python-docs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/claude-agent-sdk-python-docs-pp-mcp ./cmd/claude-agent-sdk-python-docs-pp-mcp

install-mcp:
	go install ./cmd/claude-agent-sdk-python-docs-pp-mcp

build-all: build build-mcp
