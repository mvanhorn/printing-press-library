.PHONY: build test lint install clean

build:
	go build -o bin/flipkart-pp-cli ./cmd/flipkart-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/flipkart-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/flipkart-pp-mcp ./cmd/flipkart-pp-mcp

install-mcp:
	go install ./cmd/flipkart-pp-mcp

build-all: build build-mcp
