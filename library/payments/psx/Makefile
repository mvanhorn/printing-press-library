.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/psx-pp-cli$(BIN_EXT) ./cmd/psx-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/psx-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/psx-pp-mcp$(BIN_EXT) ./cmd/psx-pp-mcp

install-mcp:
	go install ./cmd/psx-pp-mcp

build-all: build build-mcp
