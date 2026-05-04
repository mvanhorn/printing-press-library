.PHONY: build test lint install clean

build:
	go build -o bin/apartments-pp-cli ./cmd/apartments-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/apartments-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/apartments-pp-mcp ./cmd/apartments-pp-mcp

install-mcp:
	go install ./cmd/apartments-pp-mcp

build-all: build build-mcp
