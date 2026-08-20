.PHONY: build test lint install clean

build:
	go build -o bin/amazon-orders-pp-cli ./cmd/amazon-orders-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/amazon-orders-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/amazon-orders-pp-mcp ./cmd/amazon-orders-pp-mcp

install-mcp:
	go install ./cmd/amazon-orders-pp-mcp

build-all: build build-mcp
