.PHONY: build test lint install clean

build:
	go build -o bin/pinecone-pp-cli ./cmd/pinecone-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pinecone-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pinecone-pp-mcp ./cmd/pinecone-pp-mcp

install-mcp:
	go install ./cmd/pinecone-pp-mcp

build-all: build build-mcp
