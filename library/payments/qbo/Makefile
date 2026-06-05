.PHONY: build test lint install clean

build:
	go build -o bin/qbo-pp-cli ./cmd/qbo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/qbo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/qbo-pp-mcp ./cmd/qbo-pp-mcp

install-mcp:
	go install ./cmd/qbo-pp-mcp

build-all: build build-mcp
