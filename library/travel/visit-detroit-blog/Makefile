.PHONY: build test lint install clean

build:
	go build -o bin/visit-detroit-blog-pp-cli ./cmd/visit-detroit-blog-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/visit-detroit-blog-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/visit-detroit-blog-pp-mcp ./cmd/visit-detroit-blog-pp-mcp

install-mcp:
	go install ./cmd/visit-detroit-blog-pp-mcp

build-all: build build-mcp
