.PHONY: build test lint install clean

build:
	go build -o bin/tripadvisor-pp-cli ./cmd/tripadvisor-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tripadvisor-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tripadvisor-pp-mcp ./cmd/tripadvisor-pp-mcp

install-mcp:
	go install ./cmd/tripadvisor-pp-mcp

build-all: build build-mcp
