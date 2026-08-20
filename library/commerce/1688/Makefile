.PHONY: build test lint install clean

build:
	go build -o bin/1688-pp-cli ./cmd/1688-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/1688-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/1688-pp-mcp ./cmd/1688-pp-mcp

install-mcp:
	go install ./cmd/1688-pp-mcp

build-all: build build-mcp
