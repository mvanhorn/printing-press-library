.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/mcpmarket-pp-cli$(BIN_EXT) ./cmd/mcpmarket-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mcpmarket-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/mcpmarket-pp-mcp$(BIN_EXT) ./cmd/mcpmarket-pp-mcp

install-mcp:
	go install ./cmd/mcpmarket-pp-mcp

build-all: build build-mcp
