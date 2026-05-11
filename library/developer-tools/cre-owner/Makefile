.PHONY: build test lint install clean

build:
	go build -o bin/cre-owner-pp-cli ./cmd/cre-owner-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cre-owner-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cre-owner-pp-mcp ./cmd/cre-owner-pp-mcp

install-mcp:
	go install ./cmd/cre-owner-pp-mcp

build-all: build build-mcp
