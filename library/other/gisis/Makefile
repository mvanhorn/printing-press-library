.PHONY: build test lint install clean

build:
	go build -o bin/gisis-pp-cli ./cmd/gisis-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gisis-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gisis-pp-mcp ./cmd/gisis-pp-mcp

install-mcp:
	go install ./cmd/gisis-pp-mcp

build-all: build build-mcp
