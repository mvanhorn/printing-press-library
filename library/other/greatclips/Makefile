.PHONY: build test lint install clean

build:
	go build -o bin/greatclips-pp-cli ./cmd/greatclips-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/greatclips-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/greatclips-pp-mcp ./cmd/greatclips-pp-mcp

install-mcp:
	go install ./cmd/greatclips-pp-mcp

build-all: build build-mcp
