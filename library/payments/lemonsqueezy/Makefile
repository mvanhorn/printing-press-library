.PHONY: build test lint install clean

build:
	go build -o bin/lemonsqueezy-pp-cli ./cmd/lemonsqueezy-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/lemonsqueezy-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/lemonsqueezy-pp-mcp ./cmd/lemonsqueezy-pp-mcp

install-mcp:
	go install ./cmd/lemonsqueezy-pp-mcp

build-all: build build-mcp
