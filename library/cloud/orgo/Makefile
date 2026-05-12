.PHONY: build test lint install clean

build:
	go build -o bin/orgo-pp-cli ./cmd/orgo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/orgo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/orgo-pp-mcp ./cmd/orgo-pp-mcp

install-mcp:
	go install ./cmd/orgo-pp-mcp

build-all: build build-mcp
