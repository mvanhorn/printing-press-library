.PHONY: build test lint install clean

build:
	go build -o bin/stripe-pp-pp-cli ./cmd/stripe-pp-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/stripe-pp-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/stripe-pp-pp-mcp ./cmd/stripe-pp-pp-mcp

install-mcp:
	go install ./cmd/stripe-pp-pp-mcp

build-all: build build-mcp
