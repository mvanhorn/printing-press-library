.PHONY: build test lint install clean

build:
	go build -o bin/judgementtw-pp-cli ./cmd/judgementtw-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/judgementtw-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/judgementtw-pp-mcp ./cmd/judgementtw-pp-mcp

install-mcp:
	go install ./cmd/judgementtw-pp-mcp

build-all: build build-mcp
