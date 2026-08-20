.PHONY: build test lint install clean

build:
	go build -o bin/openalex-pp-cli ./cmd/openalex-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/openalex-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/openalex-pp-mcp ./cmd/openalex-pp-mcp

install-mcp:
	go install ./cmd/openalex-pp-mcp

build-all: build build-mcp
