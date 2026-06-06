.PHONY: build test lint install clean

build:
	go build -o bin/shopping-pp-cli ./cmd/shopping-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/shopping-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/shopping-pp-mcp ./cmd/shopping-pp-mcp

install-mcp:
	go install ./cmd/shopping-pp-mcp

build-all: build build-mcp
