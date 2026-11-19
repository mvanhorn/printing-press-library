.PHONY: build test lint install clean build-mcp install-mcp build-all

# Version is read from manifest.json and injected via ldflags so the binaries
# report the real release version instead of the hardcoded source defaults.
VERSION := $(shell sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' manifest.json | head -1)
CLI_LDFLAGS := -X 'readwise-pp-cli/internal/cli.version=$(VERSION)'
MCP_LDFLAGS := -X 'main.version=$(VERSION)'
BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -ldflags "$(CLI_LDFLAGS)" -o bin/readwise-pp-cli$(BIN_EXT) ./cmd/readwise-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install -ldflags "$(CLI_LDFLAGS)" ./cmd/readwise-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -ldflags "$(MCP_LDFLAGS)" -o bin/readwise-pp-mcp$(BIN_EXT) ./cmd/readwise-pp-mcp

install-mcp:
	go install -ldflags "$(MCP_LDFLAGS)" ./cmd/readwise-pp-mcp

build-all: build build-mcp
