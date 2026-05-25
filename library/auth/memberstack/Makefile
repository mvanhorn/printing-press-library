.PHONY: build test lint install clean

build:
	go build -o bin/memberstack-pp-cli ./cmd/memberstack-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/memberstack-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/memberstack-pp-mcp ./cmd/memberstack-pp-mcp

install-mcp:
	go install ./cmd/memberstack-pp-mcp

build-all: build build-mcp
