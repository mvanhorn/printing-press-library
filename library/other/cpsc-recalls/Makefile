.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/cpsc-recalls-pp-cli$(BIN_EXT) ./cmd/cpsc-recalls-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cpsc-recalls-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cpsc-recalls-pp-mcp$(BIN_EXT) ./cmd/cpsc-recalls-pp-mcp

install-mcp:
	go install ./cmd/cpsc-recalls-pp-mcp

build-all: build build-mcp
