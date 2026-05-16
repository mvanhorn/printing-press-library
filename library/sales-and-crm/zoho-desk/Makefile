.PHONY: build test lint install clean

build:
	go build -o bin/zoho-desk-pp-cli ./cmd/zoho-desk-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zoho-desk-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zoho-desk-pp-mcp ./cmd/zoho-desk-pp-mcp

install-mcp:
	go install ./cmd/zoho-desk-pp-mcp

build-all: build build-mcp
