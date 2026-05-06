.PHONY: build test lint install clean

build:
	go build -o bin/mercury-pp-cli ./cmd/mercury-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mercury-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/mercury-pp-mcp ./cmd/mercury-pp-mcp

install-mcp:
	go install ./cmd/mercury-pp-mcp

build-all: build build-mcp
