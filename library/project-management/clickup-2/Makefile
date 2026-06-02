.PHONY: build test lint install clean

build:
	go build -o bin/clickup-2-pp-cli ./cmd/clickup-2-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/clickup-2-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/clickup-2-pp-mcp ./cmd/clickup-2-pp-mcp

install-mcp:
	go install ./cmd/clickup-2-pp-mcp

build-all: build build-mcp
