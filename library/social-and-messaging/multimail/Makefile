.PHONY: build test lint install clean

build:
	go build -o bin/multimail-pp-cli ./cmd/multimail-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/multimail-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/multimail-pp-mcp ./cmd/multimail-pp-mcp

install-mcp:
	go install ./cmd/multimail-pp-mcp

build-all: build build-mcp
