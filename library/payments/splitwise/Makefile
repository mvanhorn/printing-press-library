.PHONY: build test lint install clean

build:
	go build -o bin/splitwise-pp-cli ./cmd/splitwise-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/splitwise-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/splitwise-pp-mcp ./cmd/splitwise-pp-mcp

install-mcp:
	go install ./cmd/splitwise-pp-mcp

build-all: build build-mcp
