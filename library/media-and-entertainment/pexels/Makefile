.PHONY: build test lint install clean

build:
	go build -o bin/pexels-pp-cli ./cmd/pexels-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pexels-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pexels-pp-mcp ./cmd/pexels-pp-mcp

install-mcp:
	go install ./cmd/pexels-pp-mcp

build-all: build build-mcp
