.PHONY: build test lint install clean

build:
	go build -o bin/ars-sicilia-pp-cli ./cmd/ars-sicilia-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ars-sicilia-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ars-sicilia-pp-mcp ./cmd/ars-sicilia-pp-mcp

install-mcp:
	go install ./cmd/ars-sicilia-pp-mcp

build-all: build build-mcp
