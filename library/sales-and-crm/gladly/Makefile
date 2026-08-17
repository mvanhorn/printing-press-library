.PHONY: build test lint install clean

build:
	go build -o bin/gladly-pp-cli ./cmd/gladly-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gladly-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gladly-pp-mcp ./cmd/gladly-pp-mcp

install-mcp:
	go install ./cmd/gladly-pp-mcp

build-all: build build-mcp
