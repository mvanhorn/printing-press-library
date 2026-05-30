.PHONY: build test lint install clean

build:
	go build -o bin/stackadapt-pp-cli ./cmd/stackadapt-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/stackadapt-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/stackadapt-pp-mcp ./cmd/stackadapt-pp-mcp

install-mcp:
	go install ./cmd/stackadapt-pp-mcp

build-all: build build-mcp
