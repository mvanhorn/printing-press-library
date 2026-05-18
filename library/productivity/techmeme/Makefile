.PHONY: build test lint install clean

build:
	go build -o bin/techmeme-pp-cli ./cmd/techmeme-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/techmeme-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/techmeme-pp-mcp ./cmd/techmeme-pp-mcp

install-mcp:
	go install ./cmd/techmeme-pp-mcp

build-all: build build-mcp
