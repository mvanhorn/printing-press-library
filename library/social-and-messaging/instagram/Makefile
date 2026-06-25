.PHONY: build test lint install clean

build:
	go build -o bin/instagram-pp-cli ./cmd/instagram-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/instagram-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/instagram-pp-mcp ./cmd/instagram-pp-mcp

install-mcp:
	go install ./cmd/instagram-pp-mcp

build-all: build build-mcp
