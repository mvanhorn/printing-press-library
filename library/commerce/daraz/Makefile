.PHONY: build test lint install clean

build:
	go build -o bin/daraz-pp-cli ./cmd/daraz-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/daraz-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/daraz-pp-mcp ./cmd/daraz-pp-mcp

install-mcp:
	go install ./cmd/daraz-pp-mcp

build-all: build build-mcp
