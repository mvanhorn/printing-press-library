.PHONY: build test lint install clean

build:
	go build -o bin/runcloud-pp-cli ./cmd/runcloud-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/runcloud-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/runcloud-pp-mcp ./cmd/runcloud-pp-mcp

install-mcp:
	go install ./cmd/runcloud-pp-mcp

build-all: build build-mcp
