.PHONY: build test lint install clean

build:
	go build -o bin/botsee-pp-cli ./cmd/botsee-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/botsee-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/botsee-pp-mcp ./cmd/botsee-pp-mcp

install-mcp:
	go install ./cmd/botsee-pp-mcp

build-all: build build-mcp
