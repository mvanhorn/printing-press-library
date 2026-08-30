.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/unipile-pp-cli$(BIN_EXT) ./cmd/unipile-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/unipile-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/unipile-pp-mcp$(BIN_EXT) ./cmd/unipile-pp-mcp

install-mcp:
	go install ./cmd/unipile-pp-mcp

build-all: build build-mcp
