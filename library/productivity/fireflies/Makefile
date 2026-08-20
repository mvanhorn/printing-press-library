.PHONY: build test lint install clean

build:
	go build -o bin/fireflies-pp-cli ./cmd/fireflies-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/fireflies-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/fireflies-pp-mcp ./cmd/fireflies-pp-mcp

install-mcp:
	go install ./cmd/fireflies-pp-mcp

build-all: build build-mcp
