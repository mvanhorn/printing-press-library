.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/kvmctl-pp-cli$(BIN_EXT) ./cmd/kvmctl-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/kvmctl-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/kvmctl-pp-mcp$(BIN_EXT) ./cmd/kvmctl-pp-mcp

install-mcp:
	go install ./cmd/kvmctl-pp-mcp

build-all: build build-mcp
