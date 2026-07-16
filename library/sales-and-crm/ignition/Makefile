.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/ignition-pp-cli$(BIN_EXT) ./cmd/ignition-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ignition-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ignition-pp-mcp$(BIN_EXT) ./cmd/ignition-pp-mcp

install-mcp:
	go install ./cmd/ignition-pp-mcp

build-all: build build-mcp
