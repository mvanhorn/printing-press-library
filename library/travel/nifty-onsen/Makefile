.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/nifty-onsen-pp-cli$(BIN_EXT) ./cmd/nifty-onsen-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nifty-onsen-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/nifty-onsen-pp-mcp$(BIN_EXT) ./cmd/nifty-onsen-pp-mcp

install-mcp:
	go install ./cmd/nifty-onsen-pp-mcp

build-all: build build-mcp
