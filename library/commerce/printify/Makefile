.PHONY: build test lint install clean

build:
	go build -o bin/printify-pp-cli ./cmd/printify-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/printify-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/printify-pp-mcp ./cmd/printify-pp-mcp

install-mcp:
	go install ./cmd/printify-pp-mcp

build-all: build build-mcp
