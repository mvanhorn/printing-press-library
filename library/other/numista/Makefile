.PHONY: build test lint install clean

build:
	go build -o bin/numista-pp-cli ./cmd/numista-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/numista-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/numista-pp-mcp ./cmd/numista-pp-mcp

install-mcp:
	go install ./cmd/numista-pp-mcp

build-all: build build-mcp
