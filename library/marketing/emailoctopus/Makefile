.PHONY: build test lint install clean

build:
	go build -o bin/emailoctopus-pp-cli ./cmd/emailoctopus-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/emailoctopus-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/emailoctopus-pp-mcp ./cmd/emailoctopus-pp-mcp

install-mcp:
	go install ./cmd/emailoctopus-pp-mcp

build-all: build build-mcp
