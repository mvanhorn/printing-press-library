.PHONY: build test lint install clean

build:
	go build -o bin/worten-pp-cli ./cmd/worten-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/worten-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/worten-pp-mcp ./cmd/worten-pp-mcp

install-mcp:
	go install ./cmd/worten-pp-mcp

build-all: build build-mcp