.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/cdc-pakistan-pp-cli$(BIN_EXT) ./cmd/cdc-pakistan-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cdc-pakistan-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cdc-pakistan-pp-mcp$(BIN_EXT) ./cmd/cdc-pakistan-pp-mcp

install-mcp:
	go install ./cmd/cdc-pakistan-pp-mcp

build-all: build build-mcp
