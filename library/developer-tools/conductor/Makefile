.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/conductor-pp-cli$(BIN_EXT) ./cmd/conductor-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/conductor-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/conductor-pp-mcp$(BIN_EXT) ./cmd/conductor-pp-mcp

install-mcp:
	go install ./cmd/conductor-pp-mcp

build-all: build build-mcp
