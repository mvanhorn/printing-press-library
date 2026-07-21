.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/name-that-ui-pp-cli$(BIN_EXT) ./cmd/name-that-ui-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/name-that-ui-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/name-that-ui-pp-mcp$(BIN_EXT) ./cmd/name-that-ui-pp-mcp

install-mcp:
	go install ./cmd/name-that-ui-pp-mcp

build-all: build build-mcp
