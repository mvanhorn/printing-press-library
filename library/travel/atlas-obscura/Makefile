.PHONY: build test lint install clean

build:
	go build -o bin/atlas-obscura-pp-cli ./cmd/atlas-obscura-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/atlas-obscura-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/atlas-obscura-pp-mcp ./cmd/atlas-obscura-pp-mcp

install-mcp:
	go install ./cmd/atlas-obscura-pp-mcp

build-all: build build-mcp
