.PHONY: build test lint install clean

build:
	go build -o bin/cloudflare-pp-cli ./cmd/cloudflare-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cloudflare-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cloudflare-pp-mcp ./cmd/cloudflare-pp-mcp

install-mcp:
	go install ./cmd/cloudflare-pp-mcp

build-all: build build-mcp
