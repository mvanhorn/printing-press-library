.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/shopper-pp-cli$(BIN_EXT) ./cmd/shopper-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/shopper-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/shopper-pp-mcp$(BIN_EXT) ./cmd/shopper-pp-mcp

install-mcp:
	go install ./cmd/shopper-pp-mcp

build-all: build build-mcp
