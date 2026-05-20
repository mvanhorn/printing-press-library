.PHONY: build test lint install clean

build:
	go build -o bin/figma-pp-cli ./cmd/figma-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/figma-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/figma-pp-mcp ./cmd/figma-pp-mcp

install-mcp:
	go install ./cmd/figma-pp-mcp

build-all: build build-mcp
