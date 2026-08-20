.PHONY: build test lint install clean

build:
	go build -o bin/supermemory-admin-pp-cli ./cmd/supermemory-admin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/supermemory-admin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/supermemory-admin-pp-mcp ./cmd/supermemory-admin-pp-mcp

install-mcp:
	go install ./cmd/supermemory-admin-pp-mcp

build-all: build build-mcp
