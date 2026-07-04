.PHONY: build test lint install clean

build:
	go build -o bin/google-play-pp-cli ./cmd/google-play-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-play-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/google-play-pp-mcp ./cmd/google-play-pp-mcp

install-mcp:
	go install ./cmd/google-play-pp-mcp

build-all: build build-mcp
