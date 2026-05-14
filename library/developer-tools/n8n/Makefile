.PHONY: build test lint install clean

build:
	go build -o bin/n8n-pp-cli ./cmd/n8n-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/n8n-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/n8n-pp-mcp ./cmd/n8n-pp-mcp

install-mcp:
	go install ./cmd/n8n-pp-mcp

build-all: build build-mcp
