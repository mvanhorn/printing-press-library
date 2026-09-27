.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/ikyu-pp-cli$(BIN_EXT) ./cmd/ikyu-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ikyu-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/ikyu-pp-mcp$(BIN_EXT) ./cmd/ikyu-pp-mcp

install-mcp:
	go install ./cmd/ikyu-pp-mcp

build-all: build build-mcp
