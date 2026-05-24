.PHONY: build test lint install clean

build:
	go build -o bin/synology-router-pp-cli ./cmd/synology-router-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/synology-router-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/synology-router-pp-mcp ./cmd/synology-router-pp-mcp

install-mcp:
	go install ./cmd/synology-router-pp-mcp

build-all: build build-mcp
