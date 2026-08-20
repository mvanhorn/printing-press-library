.PHONY: build test lint install clean

build:
	go build -o bin/substack-pp-cli ./cmd/substack-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/substack-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/substack-pp-mcp ./cmd/substack-pp-mcp

install-mcp:
	go install ./cmd/substack-pp-mcp

build-all: build build-mcp
