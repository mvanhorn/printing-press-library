.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/faa-registry-pp-cli$(BIN_EXT) ./cmd/faa-registry-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/faa-registry-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/faa-registry-pp-mcp$(BIN_EXT) ./cmd/faa-registry-pp-mcp

install-mcp:
	go install ./cmd/faa-registry-pp-mcp

build-all: build build-mcp
