.PHONY: build test lint install clean

build:
	go build -o bin/riverside-fm-pp-cli ./cmd/riverside-fm-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/riverside-fm-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/riverside-fm-pp-mcp ./cmd/riverside-fm-pp-mcp

install-mcp:
	go install ./cmd/riverside-fm-pp-mcp

build-all: build build-mcp
