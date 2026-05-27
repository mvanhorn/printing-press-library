.PHONY: build test lint install clean

build:
	go build -o bin/bse-filings-pp-cli ./cmd/bse-filings-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bse-filings-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bse-filings-pp-mcp ./cmd/bse-filings-pp-mcp

install-mcp:
	go install ./cmd/bse-filings-pp-mcp

build-all: build build-mcp
