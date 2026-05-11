.PHONY: build test lint install clean

build:
	go build -o bin/uspto-patents-pp-cli ./cmd/uspto-patents-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/uspto-patents-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/uspto-patents-pp-mcp ./cmd/uspto-patents-pp-mcp

install-mcp:
	go install ./cmd/uspto-patents-pp-mcp

build-all: build build-mcp
