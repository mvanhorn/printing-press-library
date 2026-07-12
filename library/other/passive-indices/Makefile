.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/passive-indices-pp-cli$(BIN_EXT) ./cmd/passive-indices-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/passive-indices-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/passive-indices-pp-mcp$(BIN_EXT) ./cmd/passive-indices-pp-mcp

install-mcp:
	go install ./cmd/passive-indices-pp-mcp

build-all: build build-mcp
