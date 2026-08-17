.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/home-assistant-pp-cli$(BIN_EXT) ./cmd/home-assistant-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/home-assistant-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/home-assistant-pp-mcp$(BIN_EXT) ./cmd/home-assistant-pp-mcp

install-mcp:
	go install ./cmd/home-assistant-pp-mcp

build-all: build build-mcp
