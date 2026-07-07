.PHONY: build test lint install clean

build:
	go build -o bin/grubhub-pp-cli ./cmd/grubhub-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/grubhub-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/grubhub-pp-mcp ./cmd/grubhub-pp-mcp

install-mcp:
	go install ./cmd/grubhub-pp-mcp

build-all: build build-mcp
