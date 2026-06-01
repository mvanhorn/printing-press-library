.PHONY: build test lint install clean

build:
	go build -o bin/seranking-pp-cli ./cmd/seranking-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/seranking-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/seranking-pp-mcp ./cmd/seranking-pp-mcp

install-mcp:
	go install ./cmd/seranking-pp-mcp

build-all: build build-mcp
