.PHONY: build test lint install clean

build:
	go build -o bin/gemini-pp-cli ./cmd/gemini-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gemini-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gemini-pp-mcp ./cmd/gemini-pp-mcp

install-mcp:
	go install ./cmd/gemini-pp-mcp

build-all: build build-mcp
