.PHONY: build test lint install clean

build:
	go build -o bin/makerworld-pp-cli ./cmd/makerworld-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/makerworld-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/makerworld-pp-mcp ./cmd/makerworld-pp-mcp

install-mcp:
	go install ./cmd/makerworld-pp-mcp

build-all: build build-mcp
