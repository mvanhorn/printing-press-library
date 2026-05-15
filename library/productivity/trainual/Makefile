.PHONY: build test lint install clean

build:
	go build -o bin/trainual-pp-cli ./cmd/trainual-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/trainual-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/trainual-pp-mcp ./cmd/trainual-pp-mcp

install-mcp:
	go install ./cmd/trainual-pp-mcp

build-all: build build-mcp
