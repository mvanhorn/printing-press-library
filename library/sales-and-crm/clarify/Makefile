.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/clarify-pp-cli$(BIN_EXT) ./cmd/clarify-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/clarify-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/clarify-pp-mcp$(BIN_EXT) ./cmd/clarify-pp-mcp

install-mcp:
	go install ./cmd/clarify-pp-mcp

build-all: build build-mcp
