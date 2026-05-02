.PHONY: build test lint install clean

build:
	go build -o bin/sentry-pp-cli ./cmd/sentry-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sentry-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sentry-pp-mcp ./cmd/sentry-pp-mcp

install-mcp:
	go install ./cmd/sentry-pp-mcp

build-all: build build-mcp
