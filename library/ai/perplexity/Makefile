.PHONY: build test lint install clean

build:
	go build -o bin/perplexity-pp-cli ./cmd/perplexity-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/perplexity-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/perplexity-pp-mcp ./cmd/perplexity-pp-mcp

install-mcp:
	go install ./cmd/perplexity-pp-mcp

build-all: build build-mcp
