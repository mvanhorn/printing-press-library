.PHONY: build test lint install clean

build:
	go build -o bin/here-now-pp-cli ./cmd/here-now-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/here-now-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/here-now-pp-mcp ./cmd/here-now-pp-mcp

install-mcp:
	go install ./cmd/here-now-pp-mcp

build-all: build build-mcp
