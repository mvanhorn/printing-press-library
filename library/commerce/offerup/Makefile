.PHONY: build test lint install clean

build:
	go build -o bin/offerup-pp-cli ./cmd/offerup-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/offerup-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/offerup-pp-mcp ./cmd/offerup-pp-mcp

install-mcp:
	go install ./cmd/offerup-pp-mcp

build-all: build build-mcp
