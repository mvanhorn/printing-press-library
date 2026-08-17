.PHONY: build test lint install clean

build:
	go build -o bin/airhint-pp-cli ./cmd/airhint-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/airhint-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/airhint-pp-mcp ./cmd/airhint-pp-mcp

install-mcp:
	go install ./cmd/airhint-pp-mcp

build-all: build build-mcp
