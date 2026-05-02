.PHONY: build test lint install clean

build:
	go build -o bin/klaviyo-pp-cli ./cmd/klaviyo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/klaviyo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/klaviyo-pp-mcp ./cmd/klaviyo-pp-mcp

install-mcp:
	go install ./cmd/klaviyo-pp-mcp

build-all: build build-mcp
