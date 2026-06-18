.PHONY: build test lint install clean

build:
	go build -o bin/affine-pp-cli ./cmd/affine-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/affine-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/affine-pp-mcp ./cmd/affine-pp-mcp

install-mcp:
	go install ./cmd/affine-pp-mcp

build-all: build build-mcp
