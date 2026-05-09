.PHONY: build test lint install clean

build:
	go build -o bin/openrouter-pp-cli ./cmd/openrouter-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/openrouter-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/openrouter-pp-mcp ./cmd/openrouter-pp-mcp

install-mcp:
	go install ./cmd/openrouter-pp-mcp

build-all: build build-mcp
