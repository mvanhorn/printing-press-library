.PHONY: build test lint install clean

build:
	go build -o bin/marginalrevolution-pp-cli ./cmd/marginalrevolution-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/marginalrevolution-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/marginalrevolution-pp-mcp ./cmd/marginalrevolution-pp-mcp

install-mcp:
	go install ./cmd/marginalrevolution-pp-mcp

build-all: build build-mcp
