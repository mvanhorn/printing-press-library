.PHONY: build test lint install clean

build:
	go build -o bin/wolt-pp-cli ./cmd/wolt-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/wolt-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/wolt-pp-mcp ./cmd/wolt-pp-mcp

install-mcp:
	go install ./cmd/wolt-pp-mcp

build-all: build build-mcp
