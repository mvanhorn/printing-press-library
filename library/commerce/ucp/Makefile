.PHONY: build test lint install clean

build:
	go build -o bin/ucp-pp-cli ./cmd/ucp-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ucp-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ucp-pp-mcp ./cmd/ucp-pp-mcp

install-mcp:
	go install ./cmd/ucp-pp-mcp

build-all: build build-mcp
