.PHONY: build test lint install clean

build:
	go build -o bin/erank-pp-cli ./cmd/erank-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/erank-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/erank-pp-mcp ./cmd/erank-pp-mcp

install-mcp:
	go install ./cmd/erank-pp-mcp

build-all: build build-mcp
