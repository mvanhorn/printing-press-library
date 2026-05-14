.PHONY: build test lint install clean

build:
	go build -o bin/pushpress-pp-cli ./cmd/pushpress-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pushpress-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pushpress-pp-mcp ./cmd/pushpress-pp-mcp

install-mcp:
	go install ./cmd/pushpress-pp-mcp

build-all: build build-mcp
