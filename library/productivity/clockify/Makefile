.PHONY: build test lint install clean

build:
	go build -o bin/clockify-pp-cli ./cmd/clockify-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/clockify-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/clockify-pp-mcp ./cmd/clockify-pp-mcp

install-mcp:
	go install ./cmd/clockify-pp-mcp

build-all: build build-mcp
