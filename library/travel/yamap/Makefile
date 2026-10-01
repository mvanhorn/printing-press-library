.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/yamap-pp-cli$(BIN_EXT) ./cmd/yamap-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/yamap-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/yamap-pp-mcp$(BIN_EXT) ./cmd/yamap-pp-mcp

install-mcp:
	go install ./cmd/yamap-pp-mcp

build-all: build build-mcp
