.PHONY: build test lint install clean

build:
	go build -o bin/stripe-pp-cli ./cmd/stripe-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/stripe-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/stripe-pp-mcp ./cmd/stripe-pp-mcp

install-mcp:
	go install ./cmd/stripe-pp-mcp

build-all: build build-mcp
