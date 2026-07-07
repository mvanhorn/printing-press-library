.PHONY: build test lint install clean

build:
	go build -o bin/expensify-pp-cli ./cmd/expensify-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/expensify-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/expensify-pp-mcp ./cmd/expensify-pp-mcp

install-mcp:
	go install ./cmd/expensify-pp-mcp

build-all: build build-mcp
