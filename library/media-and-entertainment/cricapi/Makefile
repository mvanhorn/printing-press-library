.PHONY: build test lint install clean

build:
	go build -o bin/cricapi-pp-cli ./cmd/cricapi-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cricapi-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cricapi-pp-mcp ./cmd/cricapi-pp-mcp

install-mcp:
	go install ./cmd/cricapi-pp-mcp

build-all: build build-mcp
