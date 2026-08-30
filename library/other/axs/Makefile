.PHONY: build test lint install clean

build:
	go build -o bin/axs-pp-cli ./cmd/axs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/axs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/axs-pp-mcp ./cmd/axs-pp-mcp

install-mcp:
	go install ./cmd/axs-pp-mcp

build-all: build build-mcp
