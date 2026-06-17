.PHONY: build test lint install clean

build:
	go build -o bin/world-bank-pp-cli ./cmd/world-bank-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/world-bank-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/world-bank-pp-mcp ./cmd/world-bank-pp-mcp

install-mcp:
	go install ./cmd/world-bank-pp-mcp

build-all: build build-mcp
