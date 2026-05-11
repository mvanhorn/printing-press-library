.PHONY: build test lint install clean

build:
	go build -o bin/unify-pp-cli ./cmd/unify-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/unify-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/unify-pp-mcp ./cmd/unify-pp-mcp

install-mcp:
	go install ./cmd/unify-pp-mcp

build-all: build build-mcp
