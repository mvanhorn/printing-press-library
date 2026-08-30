.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/fish-audio-pp-cli$(BIN_EXT) ./cmd/fish-audio-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/fish-audio-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/fish-audio-pp-mcp$(BIN_EXT) ./cmd/fish-audio-pp-mcp

install-mcp:
	go install ./cmd/fish-audio-pp-mcp

build-all: build build-mcp
