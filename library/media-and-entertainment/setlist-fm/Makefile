.PHONY: build test lint install clean

build:
	go build -o bin/setlist-fm-pp-cli ./cmd/setlist-fm-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/setlist-fm-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/setlist-fm-pp-mcp ./cmd/setlist-fm-pp-mcp

install-mcp:
	go install ./cmd/setlist-fm-pp-mcp

build-all: build build-mcp
