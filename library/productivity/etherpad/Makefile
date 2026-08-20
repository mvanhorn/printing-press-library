.PHONY: build test lint install clean

build:
	go build -o bin/etherpad-pp-cli ./cmd/etherpad-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/etherpad-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/etherpad-pp-mcp ./cmd/etherpad-pp-mcp

install-mcp:
	go install ./cmd/etherpad-pp-mcp

build-all: build build-mcp
