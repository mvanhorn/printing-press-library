.PHONY: build test lint install clean

build:
	go build -o bin/huggingface-pp-cli ./cmd/huggingface-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/huggingface-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/huggingface-pp-mcp ./cmd/huggingface-pp-mcp

install-mcp:
	go install ./cmd/huggingface-pp-mcp

build-all: build build-mcp
