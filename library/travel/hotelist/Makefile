.PHONY: build test lint install clean

build:
	go build -o bin/hotelist-pp-cli ./cmd/hotelist-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/hotelist-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/hotelist-pp-mcp ./cmd/hotelist-pp-mcp

install-mcp:
	go install ./cmd/hotelist-pp-mcp

build-all: build build-mcp
