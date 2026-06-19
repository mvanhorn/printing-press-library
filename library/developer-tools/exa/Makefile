.PHONY: build test lint install clean

build:
	go build -o bin/exa-pp-cli ./cmd/exa-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/exa-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/exa-pp-mcp ./cmd/exa-pp-mcp

install-mcp:
	go install ./cmd/exa-pp-mcp

build-all: build build-mcp
