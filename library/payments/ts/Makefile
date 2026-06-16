.PHONY: build test lint install clean

build:
	go build -o bin/ts-pp-cli ./cmd/ts-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ts-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ts-pp-mcp ./cmd/ts-pp-mcp

install-mcp:
	go install ./cmd/ts-pp-mcp

build-all: build build-mcp
