.PHONY: build test lint install clean

build:
	go build -o bin/twelvelabs-pp-cli ./cmd/twelvelabs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/twelvelabs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/twelvelabs-pp-mcp ./cmd/twelvelabs-pp-mcp

install-mcp:
	go install ./cmd/twelvelabs-pp-mcp

build-all: build build-mcp
