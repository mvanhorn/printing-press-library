.PHONY: build test lint install clean

build:
	go build -o bin/yt-studio-pp-cli ./cmd/yt-studio-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/yt-studio-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/yt-studio-pp-mcp ./cmd/yt-studio-pp-mcp

install-mcp:
	go install ./cmd/yt-studio-pp-mcp

build-all: build build-mcp
