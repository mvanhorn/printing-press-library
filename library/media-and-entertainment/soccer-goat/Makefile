.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/soccer-goat-pp-cli$(BIN_EXT) ./cmd/soccer-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/soccer-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/soccer-goat-pp-mcp$(BIN_EXT) ./cmd/soccer-goat-pp-mcp

install-mcp:
	go install ./cmd/soccer-goat-pp-mcp

build-all: build build-mcp
