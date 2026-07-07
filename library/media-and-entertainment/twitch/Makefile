.PHONY: build test lint install clean

build:
	go build -o bin/twitch-pp-cli ./cmd/twitch-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/twitch-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/twitch-pp-mcp ./cmd/twitch-pp-mcp

install-mcp:
	go install ./cmd/twitch-pp-mcp

build-all: build build-mcp
