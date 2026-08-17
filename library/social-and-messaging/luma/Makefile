.PHONY: build test lint install clean

build:
	go build -o bin/luma-pp-cli ./cmd/luma-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/luma-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/luma-pp-mcp ./cmd/luma-pp-mcp

install-mcp:
	go install ./cmd/luma-pp-mcp

build-all: build build-mcp
