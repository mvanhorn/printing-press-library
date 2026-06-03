.PHONY: build test lint install clean

build:
	go build -o bin/perfumenz-pp-cli ./cmd/perfumenz-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/perfumenz-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/perfumenz-pp-mcp ./cmd/perfumenz-pp-mcp

install-mcp:
	go install ./cmd/perfumenz-pp-mcp

build-all: build build-mcp
