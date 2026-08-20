.PHONY: build test lint install clean

build:
	go build -o bin/tesla-pp-cli ./cmd/tesla-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tesla-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tesla-pp-mcp ./cmd/tesla-pp-mcp

install-mcp:
	go install ./cmd/tesla-pp-mcp

build-all: build build-mcp
