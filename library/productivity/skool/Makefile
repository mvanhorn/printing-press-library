.PHONY: build test lint install clean

build:
	go build -o bin/skool-pp-cli ./cmd/skool-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/skool-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/skool-pp-mcp ./cmd/skool-pp-mcp

install-mcp:
	go install ./cmd/skool-pp-mcp

build-all: build build-mcp
