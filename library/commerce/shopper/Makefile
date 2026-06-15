.PHONY: build test lint install clean

build:
	go build -o bin/shopper-pp-cli ./cmd/shopper-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/shopper-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/shopper-pp-mcp ./cmd/shopper-pp-mcp

install-mcp:
	go install ./cmd/shopper-pp-mcp

build-all: build build-mcp
