.PHONY: build test lint install clean

build:
	go build -o bin/whoop-pp-cli ./cmd/whoop-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/whoop-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/whoop-pp-mcp ./cmd/whoop-pp-mcp

install-mcp:
	go install ./cmd/whoop-pp-mcp

build-all: build build-mcp
