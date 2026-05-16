.PHONY: build test lint install clean

build:
	go build -o bin/harvest-pp-cli ./cmd/harvest-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/harvest-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/harvest-pp-mcp ./cmd/harvest-pp-mcp

install-mcp:
	go install ./cmd/harvest-pp-mcp

build-all: build build-mcp
