.PHONY: build test lint install clean

build:
	go build -o bin/zapmail-pp-cli ./cmd/zapmail-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zapmail-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zapmail-pp-mcp ./cmd/zapmail-pp-mcp

install-mcp:
	go install ./cmd/zapmail-pp-mcp

build-all: build build-mcp
