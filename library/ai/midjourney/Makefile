.PHONY: build test lint install clean

build:
	go build -o bin/midjourney-pp-cli ./cmd/midjourney-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/midjourney-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/midjourney-pp-mcp ./cmd/midjourney-pp-mcp

install-mcp:
	go install ./cmd/midjourney-pp-mcp

build-all: build build-mcp
