.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/circle-pp-cli$(BIN_EXT) ./cmd/circle-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/circle-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/circle-pp-mcp$(BIN_EXT) ./cmd/circle-pp-mcp

install-mcp:
	go install ./cmd/circle-pp-mcp

build-all: build build-mcp
