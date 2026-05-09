.PHONY: build test lint install clean

build:
	go build -o bin/zylos-pp-cli ./cmd/zylos-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zylos-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zylos-pp-mcp ./cmd/zylos-pp-mcp

install-mcp:
	go install ./cmd/zylos-pp-mcp

build-all: build build-mcp
