.PHONY: build test lint install clean

build:
	go build -o bin/gmail-pp-cli ./cmd/gmail-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gmail-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gmail-pp-mcp ./cmd/gmail-pp-mcp

install-mcp:
	go install ./cmd/gmail-pp-mcp

build-all: build build-mcp
