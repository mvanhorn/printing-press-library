.PHONY: build test lint install clean

build:
	go build -o bin/outlook-email-pp-cli ./cmd/outlook-email-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/outlook-email-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/outlook-email-pp-mcp ./cmd/outlook-email-pp-mcp

install-mcp:
	go install ./cmd/outlook-email-pp-mcp

build-all: build build-mcp
