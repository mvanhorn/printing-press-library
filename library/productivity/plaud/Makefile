.PHONY: build test lint install clean

build:
	go build -o bin/plaud-pp-cli ./cmd/plaud-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/plaud-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/plaud-pp-mcp ./cmd/plaud-pp-mcp

install-mcp:
	go install ./cmd/plaud-pp-mcp

build-all: build build-mcp
