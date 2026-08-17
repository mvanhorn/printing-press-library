.PHONY: build test lint install clean

build:
	go build -o bin/gl-inet-pp-cli ./cmd/gl-inet-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gl-inet-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gl-inet-pp-mcp ./cmd/gl-inet-pp-mcp

install-mcp:
	go install ./cmd/gl-inet-pp-mcp

build-all: build build-mcp
