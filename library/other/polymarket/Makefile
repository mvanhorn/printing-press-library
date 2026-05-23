.PHONY: build test lint install clean

build:
	go build -o bin/polymarket-pp-cli ./cmd/polymarket-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/polymarket-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/polymarket-pp-mcp ./cmd/polymarket-pp-mcp

install-mcp:
	go install ./cmd/polymarket-pp-mcp

build-all: build build-mcp
