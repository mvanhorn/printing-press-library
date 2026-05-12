.PHONY: build test lint install clean

build:
	go build -o bin/seykota-pp-cli ./cmd/seykota-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/seykota-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/seykota-pp-mcp ./cmd/seykota-pp-mcp

install-mcp:
	go install ./cmd/seykota-pp-mcp

build-all: build build-mcp
