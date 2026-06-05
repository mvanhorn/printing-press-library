.PHONY: build test lint install clean

build:
	go build -o bin/durianpay-pp-cli ./cmd/durianpay-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/durianpay-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/durianpay-pp-mcp ./cmd/durianpay-pp-mcp

install-mcp:
	go install ./cmd/durianpay-pp-mcp

build-all: build build-mcp
