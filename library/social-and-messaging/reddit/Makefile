.PHONY: build test lint install clean

build:
	go build -o bin/reddit-pp-cli ./cmd/reddit-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/reddit-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/reddit-pp-mcp ./cmd/reddit-pp-mcp

install-mcp:
	go install ./cmd/reddit-pp-mcp

build-all: build build-mcp
