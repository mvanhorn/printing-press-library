.PHONY: build test lint install clean

build:
	go build -o bin/spotify-pp-cli ./cmd/spotify-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/spotify-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/spotify-pp-mcp ./cmd/spotify-pp-mcp

install-mcp:
	go install ./cmd/spotify-pp-mcp

build-all: build build-mcp
