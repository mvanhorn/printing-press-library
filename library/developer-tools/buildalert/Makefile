.PHONY: build test lint install clean

build:
	go build -o bin/buildalert-pp-cli ./cmd/buildalert-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/buildalert-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/buildalert-pp-mcp ./cmd/buildalert-pp-mcp

install-mcp:
	go install ./cmd/buildalert-pp-mcp

build-all: build build-mcp
