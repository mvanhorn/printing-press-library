.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/clay-pp-cli$(BIN_EXT) ./cmd/clay-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/clay-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/clay-pp-mcp$(BIN_EXT) ./cmd/clay-pp-mcp

install-mcp:
	go install ./cmd/clay-pp-mcp

build-all: build build-mcp
