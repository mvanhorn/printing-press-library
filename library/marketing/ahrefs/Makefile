.PHONY: build test lint install clean

build:
	go build -o bin/ahrefs-pp-cli ./cmd/ahrefs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ahrefs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ahrefs-pp-mcp ./cmd/ahrefs-pp-mcp

install-mcp:
	go install ./cmd/ahrefs-pp-mcp

build-all: build build-mcp
