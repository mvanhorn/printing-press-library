.PHONY: build test lint install clean

build:
	go build -o bin/openfda-pp-cli ./cmd/openfda-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/openfda-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/openfda-pp-mcp ./cmd/openfda-pp-mcp

install-mcp:
	go install ./cmd/openfda-pp-mcp

build-all: build build-mcp
