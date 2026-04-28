.PHONY: build test lint install clean

build:
	go build -o bin/spotify-web-pp-cli ./cmd/spotify-web-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/spotify-web-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/spotify-web-pp-mcp ./cmd/spotify-web-pp-mcp

install-mcp:
	go install ./cmd/spotify-web-pp-mcp

build-all: build build-mcp
