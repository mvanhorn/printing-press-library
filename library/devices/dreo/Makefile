.PHONY: build test lint install clean

build:
	go build -o bin/dreo-pp-cli ./cmd/dreo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/dreo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/dreo-pp-mcp ./cmd/dreo-pp-mcp

install-mcp:
	go install ./cmd/dreo-pp-mcp

build-all: build build-mcp
