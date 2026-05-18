.PHONY: build test lint install clean

build:
	go build -o bin/gumroad-pp-cli ./cmd/gumroad-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gumroad-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gumroad-pp-mcp ./cmd/gumroad-pp-mcp

install-mcp:
	go install ./cmd/gumroad-pp-mcp

build-all: build build-mcp
