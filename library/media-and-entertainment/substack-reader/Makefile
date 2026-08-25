.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/substack-reader-pp-cli$(BIN_EXT) ./cmd/substack-reader-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/substack-reader-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/substack-reader-pp-mcp$(BIN_EXT) ./cmd/substack-reader-pp-mcp

install-mcp:
	go install ./cmd/substack-reader-pp-mcp

build-all: build build-mcp
