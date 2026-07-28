.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/wpengine-pp-cli$(BIN_EXT) ./cmd/wpengine-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/wpengine-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/wpengine-pp-mcp$(BIN_EXT) ./cmd/wpengine-pp-mcp

install-mcp:
	go install ./cmd/wpengine-pp-mcp

build-all: build build-mcp
