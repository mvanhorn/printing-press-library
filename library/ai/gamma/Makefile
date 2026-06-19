.PHONY: build test lint install clean

build:
	go build -o bin/gamma-pp-cli ./cmd/gamma-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gamma-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gamma-pp-mcp ./cmd/gamma-pp-mcp

install-mcp:
	go install ./cmd/gamma-pp-mcp

build-all: build build-mcp
