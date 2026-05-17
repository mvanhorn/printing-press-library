.PHONY: build test lint install clean

build:
	go build -o bin/catastro-pp-cli ./cmd/catastro-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/catastro-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/catastro-pp-mcp ./cmd/catastro-pp-mcp

install-mcp:
	go install ./cmd/catastro-pp-mcp

build-all: build build-mcp
