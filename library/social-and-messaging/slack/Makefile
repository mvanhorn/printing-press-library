.PHONY: build test lint install clean

build:
	go build -o bin/slack-pp-cli ./cmd/slack-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/slack-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/slack-pp-mcp ./cmd/slack-pp-mcp

install-mcp:
	go install ./cmd/slack-pp-mcp

build-all: build build-mcp
