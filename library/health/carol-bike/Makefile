.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/carol-bike-pp-cli$(BIN_EXT) ./cmd/carol-bike-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/carol-bike-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/carol-bike-pp-mcp$(BIN_EXT) ./cmd/carol-bike-pp-mcp

install-mcp:
	go install ./cmd/carol-bike-pp-mcp

build-all: build build-mcp
