.PHONY: build test lint install clean

build:
	go build -o bin/ufo-pp-cli ./cmd/ufo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ufo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ufo-pp-mcp ./cmd/ufo-pp-mcp

install-mcp:
	go install ./cmd/ufo-pp-mcp

build-all: build build-mcp
