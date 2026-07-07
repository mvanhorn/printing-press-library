.PHONY: build test lint install clean

build:
	go build -o bin/motohunt-pp-cli ./cmd/motohunt-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/motohunt-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/motohunt-pp-mcp ./cmd/motohunt-pp-mcp

install-mcp:
	go install ./cmd/motohunt-pp-mcp

build-all: build build-mcp
