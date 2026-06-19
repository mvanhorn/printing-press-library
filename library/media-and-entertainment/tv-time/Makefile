.PHONY: build test lint install clean

build:
	go build -o bin/tv-time-pp-cli ./cmd/tv-time-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tv-time-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tv-time-pp-mcp ./cmd/tv-time-pp-mcp

install-mcp:
	go install ./cmd/tv-time-pp-mcp

build-all: build build-mcp
