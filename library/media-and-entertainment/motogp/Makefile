.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/motogp-pp-cli$(BIN_EXT) ./cmd/motogp-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/motogp-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/motogp-pp-mcp$(BIN_EXT) ./cmd/motogp-pp-mcp

install-mcp:
	go install ./cmd/motogp-pp-mcp

build-all: build build-mcp
