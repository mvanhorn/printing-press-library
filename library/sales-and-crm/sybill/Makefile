.PHONY: build test lint install clean

build:
	go build -o bin/sybill-pp-cli ./cmd/sybill-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sybill-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sybill-pp-mcp ./cmd/sybill-pp-mcp

install-mcp:
	go install ./cmd/sybill-pp-mcp

build-all: build build-mcp
