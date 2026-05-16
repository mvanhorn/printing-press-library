.PHONY: build test lint install clean

build:
	go build -o bin/youtube-creator-pp-cli ./cmd/youtube-creator-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/youtube-creator-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/youtube-creator-pp-mcp ./cmd/youtube-creator-pp-mcp

install-mcp:
	go install ./cmd/youtube-creator-pp-mcp

build-all: build build-mcp
