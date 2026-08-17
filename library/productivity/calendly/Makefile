.PHONY: build test lint install clean

build:
	go build -o bin/calendly-pp-cli ./cmd/calendly-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/calendly-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/calendly-pp-mcp ./cmd/calendly-pp-mcp

install-mcp:
	go install ./cmd/calendly-pp-mcp

build-all: build build-mcp
