.PHONY: build test lint install clean

build:
	go build -o bin/halopsa-pp-cli ./cmd/halopsa-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/halopsa-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/halopsa-pp-mcp ./cmd/halopsa-pp-mcp

install-mcp:
	go install ./cmd/halopsa-pp-mcp

build-all: build build-mcp
