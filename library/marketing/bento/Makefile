.PHONY: build test lint install clean

build:
	go build -o bin/bento-pp-cli ./cmd/bento-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bento-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bento-pp-mcp ./cmd/bento-pp-mcp

install-mcp:
	go install ./cmd/bento-pp-mcp

build-all: build build-mcp
