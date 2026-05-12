.PHONY: build test lint install clean

build:
	go build -o bin/bls-pp-cli ./cmd/bls-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bls-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bls-pp-mcp ./cmd/bls-pp-mcp

install-mcp:
	go install ./cmd/bls-pp-mcp

build-all: build build-mcp
