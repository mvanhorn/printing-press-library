.PHONY: build test lint install clean

build:
	go build -o bin/uk-train-goat-pp-cli ./cmd/uk-train-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/uk-train-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/uk-train-goat-pp-mcp ./cmd/uk-train-goat-pp-mcp

install-mcp:
	go install ./cmd/uk-train-goat-pp-mcp

build-all: build build-mcp
