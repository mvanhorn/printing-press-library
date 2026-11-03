.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/arcmira-pp-cli$(BIN_EXT) ./cmd/arcmira-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/arcmira-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/arcmira-pp-mcp$(BIN_EXT) ./cmd/arcmira-pp-mcp

install-mcp:
	go install ./cmd/arcmira-pp-mcp

build-all: build build-mcp
