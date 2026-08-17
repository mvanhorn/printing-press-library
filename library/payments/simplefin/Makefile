.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/simplefin-pp-cli$(BIN_EXT) ./cmd/simplefin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/simplefin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/simplefin-pp-mcp$(BIN_EXT) ./cmd/simplefin-pp-mcp

install-mcp:
	go install ./cmd/simplefin-pp-mcp

build-all: build build-mcp
