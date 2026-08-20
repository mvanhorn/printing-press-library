.PHONY: build test lint install clean

build:
	go build -o bin/openipa-pp-cli ./cmd/openipa-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/openipa-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/openipa-pp-mcp ./cmd/openipa-pp-mcp

install-mcp:
	go install ./cmd/openipa-pp-mcp

build-all: build build-mcp
