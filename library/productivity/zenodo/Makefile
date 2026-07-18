.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/zenodo-pp-cli$(BIN_EXT) ./cmd/zenodo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zenodo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zenodo-pp-mcp$(BIN_EXT) ./cmd/zenodo-pp-mcp

install-mcp:
	go install ./cmd/zenodo-pp-mcp

build-all: build build-mcp
