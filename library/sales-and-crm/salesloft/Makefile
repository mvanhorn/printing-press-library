.PHONY: build test lint install clean

build:
	go build -o bin/salesloft-pp-cli ./cmd/salesloft-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/salesloft-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/salesloft-pp-mcp ./cmd/salesloft-pp-mcp

install-mcp:
	go install ./cmd/salesloft-pp-mcp

build-all: build build-mcp
