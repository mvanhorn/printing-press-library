.PHONY: build test lint install clean

build:
	go build -o bin/dataforseo-pp-cli ./cmd/dataforseo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/dataforseo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/dataforseo-pp-mcp ./cmd/dataforseo-pp-mcp

install-mcp:
	go install ./cmd/dataforseo-pp-mcp

build-all: build build-mcp
