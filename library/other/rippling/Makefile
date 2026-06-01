.PHONY: build test lint install clean

build:
	go build -o bin/rippling-pp-cli ./cmd/rippling-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/rippling-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/rippling-pp-mcp ./cmd/rippling-pp-mcp

install-mcp:
	go install ./cmd/rippling-pp-mcp

build-all: build build-mcp
