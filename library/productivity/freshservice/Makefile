.PHONY: build test lint install clean

build:
	go build -o bin/freshservice-pp-cli ./cmd/freshservice-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/freshservice-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/freshservice-pp-mcp ./cmd/freshservice-pp-mcp

install-mcp:
	go install ./cmd/freshservice-pp-mcp

build-all: build build-mcp
