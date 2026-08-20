.PHONY: build test lint install clean

build:
	go build -o bin/nylas-pp-cli ./cmd/nylas-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nylas-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/nylas-pp-mcp ./cmd/nylas-pp-mcp

install-mcp:
	go install ./cmd/nylas-pp-mcp

build-all: build build-mcp
