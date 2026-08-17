.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/crates-io-pp-cli$(BIN_EXT) ./cmd/crates-io-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/crates-io-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/crates-io-pp-mcp$(BIN_EXT) ./cmd/crates-io-pp-mcp

install-mcp:
	go install ./cmd/crates-io-pp-mcp

build-all: build build-mcp
