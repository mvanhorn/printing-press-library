.PHONY: build test lint install clean

build:
	go build -o bin/toodledo-pp-cli ./cmd/toodledo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/toodledo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/toodledo-pp-mcp ./cmd/toodledo-pp-mcp

install-mcp:
	go install ./cmd/toodledo-pp-mcp

build-all: build build-mcp
