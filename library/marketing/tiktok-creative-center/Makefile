.PHONY: build test lint install clean

build:
	go build -o bin/tiktok-creative-center-pp-cli ./cmd/tiktok-creative-center-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tiktok-creative-center-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tiktok-creative-center-pp-mcp ./cmd/tiktok-creative-center-pp-mcp

install-mcp:
	go install ./cmd/tiktok-creative-center-pp-mcp

build-all: build build-mcp
