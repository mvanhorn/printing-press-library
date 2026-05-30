.PHONY: build test lint install clean

build:
	go build -o bin/gfw-pp-cli ./cmd/gfw-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gfw-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gfw-pp-mcp ./cmd/gfw-pp-mcp

install-mcp:
	go install ./cmd/gfw-pp-mcp

build-all: build build-mcp
