.PHONY: build test lint install clean

build:
	go build -o bin/gainsight-pp-cli ./cmd/gainsight-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gainsight-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gainsight-pp-mcp ./cmd/gainsight-pp-mcp

install-mcp:
	go install ./cmd/gainsight-pp-mcp

build-all: build build-mcp
