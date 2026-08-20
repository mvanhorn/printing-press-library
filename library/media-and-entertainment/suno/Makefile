.PHONY: build test lint install clean

build:
	go build -o bin/suno-pp-cli ./cmd/suno-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/suno-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/suno-pp-mcp ./cmd/suno-pp-mcp

install-mcp:
	go install ./cmd/suno-pp-mcp

build-all: build build-mcp
