.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/respondio-pp-cli$(BIN_EXT) ./cmd/respondio-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/respondio-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/respondio-pp-mcp$(BIN_EXT) ./cmd/respondio-pp-mcp

install-mcp:
	go install ./cmd/respondio-pp-mcp

build-all: build build-mcp
