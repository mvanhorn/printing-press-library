.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/nepra-pp-cli$(BIN_EXT) ./cmd/nepra-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nepra-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/nepra-pp-mcp$(BIN_EXT) ./cmd/nepra-pp-mcp

install-mcp:
	go install ./cmd/nepra-pp-mcp

build-all: build build-mcp
