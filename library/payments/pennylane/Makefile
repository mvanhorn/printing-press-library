.PHONY: build test lint install clean

build:
	go build -o bin/accounting-pp-cli ./cmd/accounting-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/accounting-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/accounting-pp-mcp ./cmd/accounting-pp-mcp

install-mcp:
	go install ./cmd/accounting-pp-mcp

build-all: build build-mcp
