.PHONY: build test lint install clean

build:
	go build -o bin/intervals-icu-pp-cli ./cmd/intervals-icu-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/intervals-icu-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/intervals-icu-pp-mcp ./cmd/intervals-icu-pp-mcp

install-mcp:
	go install ./cmd/intervals-icu-pp-mcp

build-all: build build-mcp
