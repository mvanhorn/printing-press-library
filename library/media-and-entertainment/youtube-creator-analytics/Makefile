.PHONY: build test lint install clean

build:
	go build -o bin/youtube-creator-analytics-pp-cli ./cmd/youtube-creator-analytics-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/youtube-creator-analytics-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/youtube-creator-analytics-pp-mcp ./cmd/youtube-creator-analytics-pp-mcp

install-mcp:
	go install ./cmd/youtube-creator-analytics-pp-mcp

build-all: build build-mcp
