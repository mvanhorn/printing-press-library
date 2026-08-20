.PHONY: build test lint install clean

build:
	go build -o bin/roadside-america-pp-cli ./cmd/roadside-america-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/roadside-america-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/roadside-america-pp-mcp ./cmd/roadside-america-pp-mcp

install-mcp:
	go install ./cmd/roadside-america-pp-mcp

build-all: build build-mcp
