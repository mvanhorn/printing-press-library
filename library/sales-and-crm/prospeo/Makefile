.PHONY: build test lint install clean

build:
	go build -o bin/prospeo-pp-cli ./cmd/prospeo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/prospeo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/prospeo-pp-mcp ./cmd/prospeo-pp-mcp

install-mcp:
	go install ./cmd/prospeo-pp-mcp

build-all: build build-mcp
