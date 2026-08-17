.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/discord-pp-cli$(BIN_EXT) ./cmd/discord-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/discord-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/discord-pp-mcp$(BIN_EXT) ./cmd/discord-pp-mcp

install-mcp:
	go install ./cmd/discord-pp-mcp

build-all: build build-mcp
