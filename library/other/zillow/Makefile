.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/zillow-pp-cli$(BIN_EXT) ./cmd/zillow-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zillow-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zillow-pp-mcp$(BIN_EXT) ./cmd/zillow-pp-mcp

install-mcp:
	go install ./cmd/zillow-pp-mcp

build-all: build build-mcp
