.PHONY: build test lint install clean

build:
	go build -o bin/diretta-pp-cli ./cmd/diretta-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/diretta-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/diretta-pp-mcp ./cmd/diretta-pp-mcp

install-mcp:
	go install ./cmd/diretta-pp-mcp

build-all: build build-mcp
