.PHONY: build test lint install clean

build:
	go build -o bin/amazon-ads-pp-cli ./cmd/amazon-ads-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/amazon-ads-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/amazon-ads-pp-mcp ./cmd/amazon-ads-pp-mcp

install-mcp:
	go install ./cmd/amazon-ads-pp-mcp

build-all: build build-mcp
