.PHONY: build test lint install clean

build:
	go build -o bin/redfin-pp-cli ./cmd/redfin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/redfin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/redfin-pp-mcp ./cmd/redfin-pp-mcp

install-mcp:
	go install ./cmd/redfin-pp-mcp

build-all: build build-mcp
