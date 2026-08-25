.PHONY: build test lint install clean

build:
	go build -o bin/catasto-pp-cli ./cmd/catasto-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/catasto-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/catasto-pp-mcp ./cmd/catasto-pp-mcp

install-mcp:
	go install ./cmd/catasto-pp-mcp

build-all: build build-mcp
