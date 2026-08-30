.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/flighty-pp-cli$(BIN_EXT) ./cmd/flighty-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/flighty-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/flighty-pp-mcp$(BIN_EXT) ./cmd/flighty-pp-mcp

install-mcp:
	go install ./cmd/flighty-pp-mcp

build-all: build build-mcp
