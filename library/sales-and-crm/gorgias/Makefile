.PHONY: build test lint install clean

build:
	go build -o bin/gorgias-pp-cli ./cmd/gorgias-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gorgias-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gorgias-pp-mcp ./cmd/gorgias-pp-mcp

install-mcp:
	go install ./cmd/gorgias-pp-mcp

build-all: build build-mcp
