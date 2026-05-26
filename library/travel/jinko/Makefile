.PHONY: build test lint install clean

build:
	go build -o bin/jinko-pp-cli ./cmd/jinko-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jinko-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/jinko-pp-mcp ./cmd/jinko-pp-mcp

install-mcp:
	go install ./cmd/jinko-pp-mcp

build-all: build build-mcp
