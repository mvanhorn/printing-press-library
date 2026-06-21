.PHONY: build test lint install clean

build:
	go build -o bin/valyu-pp-cli ./cmd/valyu-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/valyu-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/valyu-pp-mcp ./cmd/valyu-pp-mcp

install-mcp:
	go install ./cmd/valyu-pp-mcp

build-all: build build-mcp
