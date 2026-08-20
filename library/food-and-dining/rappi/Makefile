.PHONY: build test lint install clean

build:
	go build -o bin/rappi-pp-cli ./cmd/rappi-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/rappi-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/rappi-pp-mcp ./cmd/rappi-pp-mcp

install-mcp:
	go install ./cmd/rappi-pp-mcp

build-all: build build-mcp
