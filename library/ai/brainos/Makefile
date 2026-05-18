.PHONY: build test lint install clean

build:
	go build -o bin/brainos-pp-cli ./cmd/brainos-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/brainos-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/brainos-pp-mcp ./cmd/brainos-pp-mcp

install-mcp:
	go install ./cmd/brainos-pp-mcp

build-all: build build-mcp
