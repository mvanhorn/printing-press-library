.PHONY: build test lint install clean

build:
	go build -o bin/chrome-history-pp-cli ./cmd/chrome-history-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/chrome-history-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/chrome-history-pp-mcp ./cmd/chrome-history-pp-mcp

install-mcp:
	go install ./cmd/chrome-history-pp-mcp

build-all: build build-mcp
