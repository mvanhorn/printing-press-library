.PHONY: build test lint install clean

build:
	go build -o bin/hotel-goat-pp-cli ./cmd/hotel-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/hotel-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/hotel-goat-pp-mcp ./cmd/hotel-goat-pp-mcp

install-mcp:
	go install ./cmd/hotel-goat-pp-mcp

build-all: build build-mcp
