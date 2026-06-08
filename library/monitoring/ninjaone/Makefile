.PHONY: build test lint install clean

build:
	go build -o bin/ninjaone-pp-cli ./cmd/ninjaone-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ninjaone-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ninjaone-pp-mcp ./cmd/ninjaone-pp-mcp

install-mcp:
	go install ./cmd/ninjaone-pp-mcp

build-all: build build-mcp
