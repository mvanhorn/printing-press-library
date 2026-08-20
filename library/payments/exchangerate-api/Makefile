.PHONY: build test lint install clean

build:
	go build -o bin/exchangerate-api-pp-cli ./cmd/exchangerate-api-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/exchangerate-api-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/exchangerate-api-pp-mcp ./cmd/exchangerate-api-pp-mcp

install-mcp:
	go install ./cmd/exchangerate-api-pp-mcp

build-all: build build-mcp
