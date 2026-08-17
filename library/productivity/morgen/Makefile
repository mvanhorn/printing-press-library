.PHONY: build test lint install clean

build:
	go build -o bin/morgen-pp-cli ./cmd/morgen-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/morgen-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/morgen-pp-mcp ./cmd/morgen-pp-mcp

install-mcp:
	go install ./cmd/morgen-pp-mcp

build-all: build build-mcp
