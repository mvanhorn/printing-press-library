.PHONY: build test lint install clean

build:
	go build -o bin/vapi-pp-cli ./cmd/vapi-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/vapi-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/vapi-pp-mcp ./cmd/vapi-pp-mcp

install-mcp:
	go install ./cmd/vapi-pp-mcp

build-all: build build-mcp
