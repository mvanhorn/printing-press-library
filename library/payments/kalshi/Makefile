.PHONY: build test lint install clean

build:
	go build -o bin/kalshi-pp-cli ./cmd/kalshi-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/kalshi-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/kalshi-pp-mcp ./cmd/kalshi-pp-mcp

install-mcp:
	go install ./cmd/kalshi-pp-mcp

build-all: build build-mcp
