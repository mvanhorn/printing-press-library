.PHONY: build test lint install clean

build:
	go build -o bin/fedex-pp-cli ./cmd/fedex-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/fedex-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/fedex-pp-mcp ./cmd/fedex-pp-mcp

install-mcp:
	go install ./cmd/fedex-pp-mcp

build-all: build build-mcp
