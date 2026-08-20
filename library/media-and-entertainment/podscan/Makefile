.PHONY: build test lint install clean

build:
	go build -o bin/podscan-pp-cli ./cmd/podscan-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/podscan-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/podscan-pp-mcp ./cmd/podscan-pp-mcp

install-mcp:
	go install ./cmd/podscan-pp-mcp

build-all: build build-mcp
