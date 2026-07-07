.PHONY: build test lint install clean

build:
	go build -o bin/copper-pp-cli ./cmd/copper-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/copper-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/copper-pp-mcp ./cmd/copper-pp-mcp

install-mcp:
	go install ./cmd/copper-pp-mcp

build-all: build build-mcp
