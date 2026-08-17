.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/v0-pp-cli$(BIN_EXT) ./cmd/v0-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/v0-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/v0-pp-mcp$(BIN_EXT) ./cmd/v0-pp-mcp

install-mcp:
	go install ./cmd/v0-pp-mcp

build-all: build build-mcp
