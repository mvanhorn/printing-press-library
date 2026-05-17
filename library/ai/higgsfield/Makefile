.PHONY: build test lint install clean

build:
	go build -o bin/higgsfield-pp-cli ./cmd/higgsfield-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/higgsfield-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/higgsfield-pp-mcp ./cmd/higgsfield-pp-mcp

install-mcp:
	go install ./cmd/higgsfield-pp-mcp

build-all: build build-mcp
