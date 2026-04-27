.PHONY: build test lint install clean

build:
	go build -o bin/jsonplaceholder-pp-cli ./cmd/jsonplaceholder-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jsonplaceholder-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/jsonplaceholder-pp-mcp ./cmd/jsonplaceholder-pp-mcp

install-mcp:
	go install ./cmd/jsonplaceholder-pp-mcp

build-all: build build-mcp
