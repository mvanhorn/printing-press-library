.PHONY: build test lint install clean

build:
	go build -o bin/workiz-pp-cli ./cmd/workiz-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/workiz-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/workiz-pp-mcp ./cmd/workiz-pp-mcp

install-mcp:
	go install ./cmd/workiz-pp-mcp

build-all: build build-mcp
