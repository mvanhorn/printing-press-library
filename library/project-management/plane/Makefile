.PHONY: build test lint install clean

build:
	go build -o bin/plane-pp-cli ./cmd/plane-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/plane-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/plane-pp-mcp ./cmd/plane-pp-mcp

install-mcp:
	go install ./cmd/plane-pp-mcp

build-all: build build-mcp
