.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/vibe-signal-pp-cli$(BIN_EXT) ./cmd/vibe-signal-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/vibe-signal-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/vibe-signal-pp-mcp$(BIN_EXT) ./cmd/vibe-signal-pp-mcp

install-mcp:
	go install ./cmd/vibe-signal-pp-mcp

build-all: build build-mcp
