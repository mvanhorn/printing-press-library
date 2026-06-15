.PHONY: build test lint install clean

build:
	go build -o bin/amazon-ae-pp-cli ./cmd/amazon-ae-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/amazon-ae-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/amazon-ae-pp-mcp ./cmd/amazon-ae-pp-mcp

install-mcp:
	go install ./cmd/amazon-ae-pp-mcp

build-all: build build-mcp
