.PHONY: build test lint install clean

build:
	go build -o bin/ibkr-pp-cli ./cmd/ibkr-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ibkr-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ibkr-pp-mcp ./cmd/ibkr-pp-mcp

install-mcp:
	go install ./cmd/ibkr-pp-mcp

build-all: build build-mcp
