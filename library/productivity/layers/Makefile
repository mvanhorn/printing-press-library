.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/layers-pp-cli$(BIN_EXT) ./cmd/layers-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/layers-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/layers-pp-mcp$(BIN_EXT) ./cmd/layers-pp-mcp

install-mcp:
	go install ./cmd/layers-pp-mcp

build-all: build build-mcp
