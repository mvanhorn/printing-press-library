.PHONY: build test lint install clean

build:
	go build -o bin/ghl-pp-cli ./cmd/ghl-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ghl-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ghl-pp-mcp ./cmd/ghl-pp-mcp

install-mcp:
	go install ./cmd/ghl-pp-mcp

build-all: build build-mcp
