.PHONY: build test lint install clean

build:
	go build -o bin/airtable-pp-cli ./cmd/airtable-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/airtable-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/airtable-pp-mcp ./cmd/airtable-pp-mcp

install-mcp:
	go install ./cmd/airtable-pp-mcp

build-all: build build-mcp
