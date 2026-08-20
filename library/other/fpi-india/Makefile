.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/fpi-india-pp-cli$(BIN_EXT) ./cmd/fpi-india-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/fpi-india-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/fpi-india-pp-mcp$(BIN_EXT) ./cmd/fpi-india-pp-mcp

install-mcp:
	go install ./cmd/fpi-india-pp-mcp

build-all: build build-mcp
