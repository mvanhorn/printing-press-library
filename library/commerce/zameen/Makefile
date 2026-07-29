.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/zameen-pp-cli$(BIN_EXT) ./cmd/zameen-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zameen-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zameen-pp-mcp$(BIN_EXT) ./cmd/zameen-pp-mcp

install-mcp:
	go install ./cmd/zameen-pp-mcp

build-all: build build-mcp
