.PHONY: build test lint install clean

build:
	go build -o bin/flowiseai-pp-cli ./cmd/flowiseai-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/flowiseai-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/flowiseai-pp-mcp ./cmd/flowiseai-pp-mcp

install-mcp:
	go install ./cmd/flowiseai-pp-mcp

build-all: build build-mcp
