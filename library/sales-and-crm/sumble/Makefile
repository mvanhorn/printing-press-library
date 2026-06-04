.PHONY: build test lint install clean

build:
	go build -o bin/sumble-pp-cli ./cmd/sumble-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sumble-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sumble-pp-mcp ./cmd/sumble-pp-mcp

install-mcp:
	go install ./cmd/sumble-pp-mcp

build-all: build build-mcp
