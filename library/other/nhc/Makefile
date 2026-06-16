.PHONY: build test lint install clean

build:
	go build -o bin/nhc-pp-cli ./cmd/nhc-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nhc-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/nhc-pp-mcp ./cmd/nhc-pp-mcp

install-mcp:
	go install ./cmd/nhc-pp-mcp

build-all: build build-mcp
