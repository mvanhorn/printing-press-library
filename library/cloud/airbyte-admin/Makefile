.PHONY: build test lint install clean

build:
	go build -o bin/airbyte-admin-pp-cli ./cmd/airbyte-admin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/airbyte-admin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/airbyte-admin-pp-mcp ./cmd/airbyte-admin-pp-mcp

install-mcp:
	go install ./cmd/airbyte-admin-pp-mcp

build-all: build build-mcp
