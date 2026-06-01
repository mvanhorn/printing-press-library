.PHONY: build test lint install clean

build:
	go build -o bin/pinterest-pp-cli ./cmd/pinterest-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pinterest-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pinterest-pp-mcp ./cmd/pinterest-pp-mcp

install-mcp:
	go install ./cmd/pinterest-pp-mcp

build-all: build build-mcp
