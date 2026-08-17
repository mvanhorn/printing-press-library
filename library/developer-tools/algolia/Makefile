.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/algolia-pp-cli$(BIN_EXT) ./cmd/algolia-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/algolia-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/algolia-pp-mcp$(BIN_EXT) ./cmd/algolia-pp-mcp

install-mcp:
	go install ./cmd/algolia-pp-mcp

build-all: build build-mcp
