.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/smartex-pp-cli$(BIN_EXT) ./cmd/smartex-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/smartex-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/smartex-pp-mcp$(BIN_EXT) ./cmd/smartex-pp-mcp

install-mcp:
	go install ./cmd/smartex-pp-mcp

build-all: build build-mcp
