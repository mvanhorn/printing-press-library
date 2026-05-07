.PHONY: build test lint install clean

build:
	go build -o bin/customer-io-pp-cli ./cmd/customer-io-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/customer-io-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/customer-io-pp-mcp ./cmd/customer-io-pp-mcp

install-mcp:
	go install ./cmd/customer-io-pp-mcp

build-all: build build-mcp
