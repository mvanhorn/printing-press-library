.PHONY: build test lint install clean

build:
	go build -o bin/techtwitter-pp-cli ./cmd/techtwitter-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/techtwitter-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/techtwitter-pp-mcp ./cmd/techtwitter-pp-mcp

install-mcp:
	go install ./cmd/techtwitter-pp-mcp

build-all: build build-mcp
