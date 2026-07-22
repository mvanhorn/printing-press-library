.PHONY: build test lint install clean

build:
	go build -o bin/framer-pp-cli ./cmd/framer-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/framer-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/framer-pp-mcp ./cmd/framer-pp-mcp

install-mcp:
	go install ./cmd/framer-pp-mcp

build-all: build build-mcp
