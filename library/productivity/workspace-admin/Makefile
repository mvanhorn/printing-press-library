.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/workspace-admin-pp-cli$(BIN_EXT) ./cmd/workspace-admin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/workspace-admin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/workspace-admin-pp-mcp$(BIN_EXT) ./cmd/workspace-admin-pp-mcp

install-mcp:
	go install ./cmd/workspace-admin-pp-mcp

build-all: build build-mcp
