.PHONY: build test lint install clean

build:
	go build -o bin/fathom-pp-cli ./cmd/fathom-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/fathom-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/fathom-pp-mcp ./cmd/fathom-pp-mcp

install-mcp:
	go install ./cmd/fathom-pp-mcp

build-all: build build-mcp
