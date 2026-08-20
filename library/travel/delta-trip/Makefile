.PHONY: build test lint install clean

build:
	go build -o bin/delta-trip-pp-cli ./cmd/delta-trip-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/delta-trip-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/delta-trip-pp-mcp ./cmd/delta-trip-pp-mcp

install-mcp:
	go install ./cmd/delta-trip-pp-mcp

build-all: build build-mcp
