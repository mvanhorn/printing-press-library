.PHONY: build test lint install clean

build:
	go build -o bin/anylist-pp-cli ./cmd/anylist-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/anylist-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/anylist-pp-mcp ./cmd/anylist-pp-mcp

install-mcp:
	go install ./cmd/anylist-pp-mcp

build-all: build build-mcp
