.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/app-store-pp-cli$(BIN_EXT) ./cmd/app-store-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/app-store-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/app-store-pp-mcp$(BIN_EXT) ./cmd/app-store-pp-mcp

install-mcp:
	go install ./cmd/app-store-pp-mcp

build-all: build build-mcp
