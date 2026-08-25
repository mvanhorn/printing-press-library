.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/quentli-pp-cli$(BIN_EXT) ./cmd/quentli-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/quentli-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/quentli-pp-mcp$(BIN_EXT) ./cmd/quentli-pp-mcp

install-mcp:
	go install ./cmd/quentli-pp-mcp

build-all: build build-mcp
