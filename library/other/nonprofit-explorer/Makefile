.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/nonprofit-explorer-pp-cli$(BIN_EXT) ./cmd/nonprofit-explorer-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nonprofit-explorer-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/nonprofit-explorer-pp-mcp$(BIN_EXT) ./cmd/nonprofit-explorer-pp-mcp

install-mcp:
	go install ./cmd/nonprofit-explorer-pp-mcp

build-all: build build-mcp
