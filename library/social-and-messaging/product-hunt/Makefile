.PHONY: build test lint install clean

build:
	go build -o bin/product-hunt-pp-cli ./cmd/product-hunt-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/product-hunt-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/product-hunt-pp-mcp ./cmd/product-hunt-pp-mcp

install-mcp:
	go install ./cmd/product-hunt-pp-mcp

build-all: build build-mcp
