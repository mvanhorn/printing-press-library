.PHONY: build test lint install clean

build:
	go build -o bin/zulip-pp-cli ./cmd/zulip-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zulip-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zulip-pp-mcp ./cmd/zulip-pp-mcp

install-mcp:
	go install ./cmd/zulip-pp-mcp

build-all: build build-mcp
