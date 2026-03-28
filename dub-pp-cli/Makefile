.PHONY: build test lint install clean

build:
	go build -o bin/dub-cli ./cmd/dub-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/dub-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/dub-mcp ./cmd/dub-mcp

install-mcp:
	go install ./cmd/dub-mcp

build-all: build build-mcp
