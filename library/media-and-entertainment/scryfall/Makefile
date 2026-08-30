.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/scryfall-pp-cli$(BIN_EXT) ./cmd/scryfall-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/scryfall-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/scryfall-pp-mcp$(BIN_EXT) ./cmd/scryfall-pp-mcp

install-mcp:
	go install ./cmd/scryfall-pp-mcp

build-all: build build-mcp
