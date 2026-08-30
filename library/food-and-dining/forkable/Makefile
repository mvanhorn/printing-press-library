.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/forkable-pp-cli$(BIN_EXT) ./cmd/forkable-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/forkable-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/forkable-pp-mcp$(BIN_EXT) ./cmd/forkable-pp-mcp

install-mcp:
	go install ./cmd/forkable-pp-mcp

build-all: build build-mcp
