.PHONY: build test lint install clean

build:
	go build -o bin/judge-me-pp-cli ./cmd/judge-me-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/judge-me-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/judge-me-pp-mcp ./cmd/judge-me-pp-mcp

install-mcp:
	go install ./cmd/judge-me-pp-mcp

build-all: build build-mcp
