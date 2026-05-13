.PHONY: build test lint install clean

build:
	go build -o bin/alphavantage-pp-cli ./cmd/alphavantage-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/alphavantage-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/alphavantage-pp-mcp ./cmd/alphavantage-pp-mcp

install-mcp:
	go install ./cmd/alphavantage-pp-mcp

build-all: build build-mcp
