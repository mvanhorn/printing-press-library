.PHONY: build test lint install clean

build:
	go build -o bin/salesforce-headless-360-pp-cli ./cmd/salesforce-headless-360-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/salesforce-headless-360-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/salesforce-headless-360-pp-mcp ./cmd/salesforce-headless-360-pp-mcp

install-mcp:
	go install ./cmd/salesforce-headless-360-pp-mcp

build-all: build build-mcp
