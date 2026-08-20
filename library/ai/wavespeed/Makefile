.PHONY: build test lint install clean

build:
	go build -o bin/wavespeed-pp-cli ./cmd/wavespeed-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/wavespeed-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/wavespeed-pp-mcp ./cmd/wavespeed-pp-mcp

install-mcp:
	go install ./cmd/wavespeed-pp-mcp

build-all: build build-mcp
