.PHONY: build test lint install clean

build:
	go build -o bin/retraction-checker-pp-cli ./cmd/retraction-checker-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/retraction-checker-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/retraction-checker-pp-mcp ./cmd/retraction-checker-pp-mcp

install-mcp:
	go install ./cmd/retraction-checker-pp-mcp

build-all: build build-mcp
