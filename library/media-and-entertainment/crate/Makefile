.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/crate-pp-cli$(BIN_EXT) ./cmd/crate-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/crate-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/crate-pp-mcp$(BIN_EXT) ./cmd/crate-pp-mcp

install-mcp:
	go install ./cmd/crate-pp-mcp

build-all: build build-mcp
