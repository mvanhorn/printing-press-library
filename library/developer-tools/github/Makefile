.PHONY: build test lint install clean

build:
	go build -o bin/github-pp-cli ./cmd/github-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/github-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/github-pp-mcp ./cmd/github-pp-mcp

install-mcp:
	go install ./cmd/github-pp-mcp

build-all: build build-mcp
