.PHONY: build test lint install clean

build:
	go build -o bin/plaud-web-pp-cli ./cmd/plaud-web-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/plaud-web-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/plaud-web-pp-mcp ./cmd/plaud-web-pp-mcp

install-mcp:
	go install ./cmd/plaud-web-pp-mcp

build-all: build build-mcp
