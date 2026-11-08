.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/milanote-pp-cli$(BIN_EXT) ./cmd/milanote-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/milanote-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/milanote-pp-mcp$(BIN_EXT) ./cmd/milanote-pp-mcp

install-mcp:
	go install ./cmd/milanote-pp-mcp

build-all: build build-mcp
