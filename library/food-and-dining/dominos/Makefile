.PHONY: build test lint install clean

build:
	go build -o bin/dominos-pp-cli ./cmd/dominos-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/dominos-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/dominos-pp-mcp ./cmd/dominos-pp-mcp

install-mcp:
	go install ./cmd/dominos-pp-mcp

build-all: build build-mcp
