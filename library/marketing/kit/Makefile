.PHONY: build test lint install clean

build:
	go build -o bin/kit-pp-cli ./cmd/kit-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/kit-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/kit-pp-mcp ./cmd/kit-pp-mcp

install-mcp:
	go install ./cmd/kit-pp-mcp

build-all: build build-mcp
