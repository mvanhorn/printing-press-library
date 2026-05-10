.PHONY: build test lint install clean

build:
	go build -o bin/ncbi-entrez-pp-cli ./cmd/ncbi-entrez-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ncbi-entrez-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ncbi-entrez-pp-mcp ./cmd/ncbi-entrez-pp-mcp

install-mcp:
	go install ./cmd/ncbi-entrez-pp-mcp

build-all: build build-mcp
