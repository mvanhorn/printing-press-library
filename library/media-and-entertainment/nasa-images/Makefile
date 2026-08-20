.PHONY: build test lint install clean

build:
	go build -o bin/nasa-images-pp-cli ./cmd/nasa-images-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nasa-images-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/nasa-images-pp-mcp ./cmd/nasa-images-pp-mcp

install-mcp:
	go install ./cmd/nasa-images-pp-mcp

build-all: build build-mcp
