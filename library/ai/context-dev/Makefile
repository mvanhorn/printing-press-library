.PHONY: build test lint install clean

build:
	go build -o bin/context-dev-pp-cli ./cmd/context-dev-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/context-dev-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/context-dev-pp-mcp ./cmd/context-dev-pp-mcp

install-mcp:
	go install ./cmd/context-dev-pp-mcp

build-all: build build-mcp
