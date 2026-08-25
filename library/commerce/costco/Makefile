.PHONY: build test lint install clean

build:
	go build -o bin/costco-pp-cli ./cmd/costco-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/costco-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/costco-pp-mcp ./cmd/costco-pp-mcp

install-mcp:
	go install ./cmd/costco-pp-mcp

build-all: build build-mcp
