.PHONY: build test lint install clean

build:
	go build -o bin/surfline-pp-cli ./cmd/surfline-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/surfline-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/surfline-pp-mcp ./cmd/surfline-pp-mcp

install-mcp:
	go install ./cmd/surfline-pp-mcp

build-all: build build-mcp
