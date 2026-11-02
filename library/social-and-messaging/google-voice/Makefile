.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/google-voice-pp-cli$(BIN_EXT) ./cmd/google-voice-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-voice-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/google-voice-pp-mcp$(BIN_EXT) ./cmd/google-voice-pp-mcp

install-mcp:
	go install ./cmd/google-voice-pp-mcp

build-all: build build-mcp
