.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/sarvam-pp-cli$(BIN_EXT) ./cmd/sarvam-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sarvam-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sarvam-pp-mcp$(BIN_EXT) ./cmd/sarvam-pp-mcp

install-mcp:
	go install ./cmd/sarvam-pp-mcp

build-all: build build-mcp
