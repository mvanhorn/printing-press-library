.PHONY: build test lint install clean

build:
	go build -o bin/artistly-pp-cli ./cmd/artistly-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/artistly-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/artistly-pp-mcp ./cmd/artistly-pp-mcp

install-mcp:
	go install ./cmd/artistly-pp-mcp

build-all: build build-mcp
