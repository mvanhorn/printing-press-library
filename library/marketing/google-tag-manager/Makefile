.PHONY: build test lint install clean

build:
	go build -o bin/google-tag-manager-pp-cli ./cmd/google-tag-manager-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-tag-manager-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/google-tag-manager-pp-mcp ./cmd/google-tag-manager-pp-mcp

install-mcp:
	go install ./cmd/google-tag-manager-pp-mcp

build-all: build build-mcp
