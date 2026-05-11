.PHONY: build test lint install clean

build:
	go build -o bin/goose-pp-cli ./cmd/goose-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/goose-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/goose-pp-mcp ./cmd/goose-pp-mcp

install-mcp:
	go install ./cmd/goose-pp-mcp

build-all: build build-mcp
