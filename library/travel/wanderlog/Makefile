.PHONY: build test lint install clean

build:
	go build -o bin/wanderlog-pp-cli ./cmd/wanderlog-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/wanderlog-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/wanderlog-pp-mcp ./cmd/wanderlog-pp-mcp

install-mcp:
	go install ./cmd/wanderlog-pp-mcp

build-all: build build-mcp
