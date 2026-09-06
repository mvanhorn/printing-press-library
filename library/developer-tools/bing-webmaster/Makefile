.PHONY: build test lint install clean

build:
	go build -o bin/bing-webmaster-pp-cli ./cmd/bing-webmaster-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bing-webmaster-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bing-webmaster-pp-mcp ./cmd/bing-webmaster-pp-mcp

install-mcp:
	go install ./cmd/bing-webmaster-pp-mcp

build-all: build build-mcp
