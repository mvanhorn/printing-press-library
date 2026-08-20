.PHONY: build test lint install clean

build:
	go build -o bin/breezedoc-pp-cli ./cmd/breezedoc-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/breezedoc-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/breezedoc-pp-mcp ./cmd/breezedoc-pp-mcp

install-mcp:
	go install ./cmd/breezedoc-pp-mcp

build-all: build build-mcp
