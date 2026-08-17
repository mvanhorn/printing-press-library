.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/printgoat-pp-cli$(BIN_EXT) ./cmd/printgoat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/printgoat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/printgoat-pp-mcp$(BIN_EXT) ./cmd/printgoat-pp-mcp

install-mcp:
	go install ./cmd/printgoat-pp-mcp

build-all: build build-mcp
