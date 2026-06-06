.PHONY: build test lint install clean

build:
	go build -o bin/substack-notes-pp-cli ./cmd/substack-notes-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/substack-notes-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/substack-notes-pp-mcp ./cmd/substack-notes-pp-mcp

install-mcp:
	go install ./cmd/substack-notes-pp-mcp

build-all: build build-mcp
