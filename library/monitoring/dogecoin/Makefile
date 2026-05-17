.PHONY: build test lint install clean

build:
	go build -o bin/dogecoin-pp-cli ./cmd/dogecoin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/dogecoin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/dogecoin-pp-mcp ./cmd/dogecoin-pp-mcp

install-mcp:
	go install ./cmd/dogecoin-pp-mcp

build-all: build build-mcp
