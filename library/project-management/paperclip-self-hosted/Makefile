.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/paperclip-pp-cli$(BIN_EXT) ./cmd/paperclip-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/paperclip-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/paperclip-pp-mcp$(BIN_EXT) ./cmd/paperclip-pp-mcp

install-mcp:
	go install ./cmd/paperclip-pp-mcp

build-all: build build-mcp
