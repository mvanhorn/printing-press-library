.PHONY: build test lint install clean

build:
	go build -o bin/clickup-pp-cli ./cmd/clickup-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/clickup-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/clickup-pp-mcp ./cmd/clickup-pp-mcp

install-mcp:
	go install ./cmd/clickup-pp-mcp

build-all: build build-mcp
