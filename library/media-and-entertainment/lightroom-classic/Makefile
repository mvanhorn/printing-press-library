.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/lightroom-classic-pp-cli$(BIN_EXT) ./cmd/lightroom-classic-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/lightroom-classic-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/lightroom-classic-pp-mcp$(BIN_EXT) ./cmd/lightroom-classic-pp-mcp

install-mcp:
	go install ./cmd/lightroom-classic-pp-mcp

build-all: build build-mcp
