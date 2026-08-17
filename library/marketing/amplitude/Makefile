.PHONY: build test lint install clean

build:
	go build -o bin/amplitude-pp-cli ./cmd/amplitude-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/amplitude-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/amplitude-pp-mcp ./cmd/amplitude-pp-mcp

install-mcp:
	go install ./cmd/amplitude-pp-mcp

build-all: build build-mcp
