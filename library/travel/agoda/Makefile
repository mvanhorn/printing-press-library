.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/agoda-pp-cli$(BIN_EXT) ./cmd/agoda-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/agoda-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/agoda-pp-mcp$(BIN_EXT) ./cmd/agoda-pp-mcp

install-mcp:
	go install ./cmd/agoda-pp-mcp

build-all: build build-mcp
