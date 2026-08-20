.PHONY: build test lint install clean

build:
	go build -o bin/github-intel-pp-cli ./cmd/github-intel-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/github-intel-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/github-intel-pp-mcp ./cmd/github-intel-pp-mcp

install-mcp:
	go install ./cmd/github-intel-pp-mcp

build-all: build build-mcp
