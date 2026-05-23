.PHONY: build test lint install clean

build:
	go build -o bin/dice-fm-pp-cli ./cmd/dice-fm-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/dice-fm-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/dice-fm-pp-mcp ./cmd/dice-fm-pp-mcp

install-mcp:
	go install ./cmd/dice-fm-pp-mcp

build-all: build build-mcp
