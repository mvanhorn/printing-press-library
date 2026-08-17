.PHONY: build test lint install clean

build:
	go build -o bin/metacritic-pp-cli ./cmd/metacritic-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/metacritic-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/metacritic-pp-mcp ./cmd/metacritic-pp-mcp

install-mcp:
	go install ./cmd/metacritic-pp-mcp

build-all: build build-mcp
