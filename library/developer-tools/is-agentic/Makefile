.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/is-agentic-pp-cli$(BIN_EXT) ./cmd/is-agentic-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/is-agentic-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/is-agentic-pp-mcp$(BIN_EXT) ./cmd/is-agentic-pp-mcp

install-mcp:
	go install ./cmd/is-agentic-pp-mcp

build-all: build build-mcp
