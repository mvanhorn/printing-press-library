.PHONY: build test lint install clean

build:
	go build -o bin/sculptok-pp-cli ./cmd/sculptok-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sculptok-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sculptok-pp-mcp ./cmd/sculptok-pp-mcp

install-mcp:
	go install ./cmd/sculptok-pp-mcp

build-all: build build-mcp
