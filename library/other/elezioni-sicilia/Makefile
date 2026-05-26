.PHONY: build test lint install clean

build:
	go build -o bin/elezioni-sicilia-pp-cli ./cmd/elezioni-sicilia-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/elezioni-sicilia-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/elezioni-sicilia-pp-mcp ./cmd/elezioni-sicilia-pp-mcp

install-mcp:
	go install ./cmd/elezioni-sicilia-pp-mcp

build-all: build build-mcp
