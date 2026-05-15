.PHONY: build test lint install clean

build:
	go build -o bin/cartesia-pp-cli ./cmd/cartesia-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cartesia-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cartesia-pp-mcp ./cmd/cartesia-pp-mcp

install-mcp:
	go install ./cmd/cartesia-pp-mcp

build-all: build build-mcp
