.PHONY: build test lint install clean

build:
	go build -o bin/1password-pp-cli ./cmd/1password-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/1password-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/1password-pp-mcp ./cmd/1password-pp-mcp

install-mcp:
	go install ./cmd/1password-pp-mcp

build-all: build build-mcp
