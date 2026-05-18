.PHONY: build test lint install clean

build:
	go build -o bin/jimmy-johns-pp-cli ./cmd/jimmy-johns-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jimmy-johns-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/jimmy-johns-pp-mcp ./cmd/jimmy-johns-pp-mcp

install-mcp:
	go install ./cmd/jimmy-johns-pp-mcp

build-all: build build-mcp
