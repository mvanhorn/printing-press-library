.PHONY: build test lint install clean

build:
	go build -o bin/pipedrive-pp-cli ./cmd/pipedrive-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pipedrive-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pipedrive-pp-mcp ./cmd/pipedrive-pp-mcp

install-mcp:
	go install ./cmd/pipedrive-pp-mcp

build-all: build build-mcp
