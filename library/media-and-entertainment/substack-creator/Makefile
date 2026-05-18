.PHONY: build test lint install clean

build:
	go build -o bin/substack-creator-pp-cli ./cmd/substack-creator-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/substack-creator-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/substack-creator-pp-mcp ./cmd/substack-creator-pp-mcp

install-mcp:
	go install ./cmd/substack-creator-pp-mcp

build-all: build build-mcp
