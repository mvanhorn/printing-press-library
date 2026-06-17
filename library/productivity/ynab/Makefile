.PHONY: build test lint install clean

build:
	go build -o bin/ynab-pp-cli ./cmd/ynab-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ynab-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ynab-pp-mcp ./cmd/ynab-pp-mcp

install-mcp:
	go install ./cmd/ynab-pp-mcp

build-all: build build-mcp
