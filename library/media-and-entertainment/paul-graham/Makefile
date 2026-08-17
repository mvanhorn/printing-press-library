.PHONY: build test lint install clean

build:
	go build -o bin/paul-graham-pp-cli ./cmd/paul-graham-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/paul-graham-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/paul-graham-pp-mcp ./cmd/paul-graham-pp-mcp

install-mcp:
	go install ./cmd/paul-graham-pp-mcp

build-all: build build-mcp
