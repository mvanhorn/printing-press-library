.PHONY: build test lint install clean

build:
	go build -o bin/chainels-pp-cli ./cmd/chainels-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/chainels-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/chainels-pp-mcp ./cmd/chainels-pp-mcp

install-mcp:
	go install ./cmd/chainels-pp-mcp

build-all: build build-mcp
