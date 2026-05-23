.PHONY: build test lint install clean

build:
	go build -o bin/servosity-pp-cli ./cmd/servosity-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/servosity-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/servosity-pp-mcp ./cmd/servosity-pp-mcp

install-mcp:
	go install ./cmd/servosity-pp-mcp

build-all: build build-mcp
