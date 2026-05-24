.PHONY: build test lint install clean

build:
	go build -o bin/synology-nas-api-pp-cli ./cmd/synology-nas-api-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/synology-nas-api-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/synology-nas-api-pp-mcp ./cmd/synology-nas-api-pp-mcp

install-mcp:
	go install ./cmd/synology-nas-api-pp-mcp

build-all: build build-mcp
