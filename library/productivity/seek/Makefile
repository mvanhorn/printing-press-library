.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/seek-pp-cli$(BIN_EXT) ./cmd/seek-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/seek-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/seek-pp-mcp$(BIN_EXT) ./cmd/seek-pp-mcp

install-mcp:
	go install ./cmd/seek-pp-mcp

build-all: build build-mcp
