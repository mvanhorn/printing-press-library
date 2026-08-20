.PHONY: build test lint install clean

build:
	go build -o bin/sec-edgar-pp-cli ./cmd/sec-edgar-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sec-edgar-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sec-edgar-pp-mcp ./cmd/sec-edgar-pp-mcp

install-mcp:
	go install ./cmd/sec-edgar-pp-mcp

build-all: build build-mcp
