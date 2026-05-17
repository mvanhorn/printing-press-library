.PHONY: build test lint install clean

build:
	go build -o bin/google-reviews-pp-cli ./cmd/google-reviews-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-reviews-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/google-reviews-pp-mcp ./cmd/google-reviews-pp-mcp

install-mcp:
	go install ./cmd/google-reviews-pp-mcp

build-all: build build-mcp
