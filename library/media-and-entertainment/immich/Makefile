.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/immich-pp-cli$(BIN_EXT) ./cmd/immich-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/immich-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/immich-pp-mcp$(BIN_EXT) ./cmd/immich-pp-mcp

install-mcp:
	go install ./cmd/immich-pp-mcp

build-all: build build-mcp
