.PHONY: build test lint install clean

build:
	go build -o bin/edgar-pp-cli ./cmd/edgar-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/edgar-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/edgar-pp-mcp ./cmd/edgar-pp-mcp

install-mcp:
	go install ./cmd/edgar-pp-mcp

build-all: build build-mcp
