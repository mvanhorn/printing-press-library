.PHONY: build test lint install clean

build:
	go build -o bin/squire-pp-cli ./cmd/squire-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/squire-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/squire-pp-mcp ./cmd/squire-pp-mcp

install-mcp:
	go install ./cmd/squire-pp-mcp

build-all: build build-mcp
