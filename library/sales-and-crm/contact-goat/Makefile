.PHONY: build test lint install clean

build:
	go build -o bin/contact-goat-pp-cli ./cmd/contact-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/contact-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/contact-goat-pp-mcp ./cmd/contact-goat-pp-mcp

install-mcp:
	go install ./cmd/contact-goat-pp-mcp

build-all: build build-mcp
