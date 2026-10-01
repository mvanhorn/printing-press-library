.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/jma-pp-cli$(BIN_EXT) ./cmd/jma-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jma-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/jma-pp-mcp$(BIN_EXT) ./cmd/jma-pp-mcp

install-mcp:
	go install ./cmd/jma-pp-mcp

build-all: build build-mcp
