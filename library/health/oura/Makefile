.PHONY: build test lint install clean

build:
	go build -o bin/oura-pp-cli ./cmd/oura-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/oura-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/oura-pp-mcp ./cmd/oura-pp-mcp

install-mcp:
	go install ./cmd/oura-pp-mcp

build-all: build build-mcp
