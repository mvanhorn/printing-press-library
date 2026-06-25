.PHONY: build test lint install clean

build:
	go build -o bin/shipsgo-pp-cli ./cmd/shipsgo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/shipsgo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/shipsgo-pp-mcp ./cmd/shipsgo-pp-mcp

install-mcp:
	go install ./cmd/shipsgo-pp-mcp

build-all: build build-mcp
