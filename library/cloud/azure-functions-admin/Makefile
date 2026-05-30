.PHONY: build test lint install clean

build:
	go build -o bin/azure-functions-admin-pp-cli ./cmd/azure-functions-admin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/azure-functions-admin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/azure-functions-admin-pp-mcp ./cmd/azure-functions-admin-pp-mcp

install-mcp:
	go install ./cmd/azure-functions-admin-pp-mcp

build-all: build build-mcp
