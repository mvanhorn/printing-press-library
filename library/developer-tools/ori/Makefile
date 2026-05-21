.PHONY: build test lint install clean

build:
	go build -o bin/ori-pp-cli ./cmd/ori-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ori-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ori-pp-mcp ./cmd/ori-pp-mcp

install-mcp:
	go install ./cmd/ori-pp-mcp

build-all: build build-mcp
