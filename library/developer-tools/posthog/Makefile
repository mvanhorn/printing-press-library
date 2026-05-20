.PHONY: build test lint install clean

build:
	go build -o bin/posthog-pp-cli ./cmd/posthog-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/posthog-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/posthog-pp-mcp ./cmd/posthog-pp-mcp

install-mcp:
	go install ./cmd/posthog-pp-mcp

build-all: build build-mcp
