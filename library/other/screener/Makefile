.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/screener-pp-cli$(BIN_EXT) ./cmd/screener-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/screener-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/screener-pp-mcp$(BIN_EXT) ./cmd/screener-pp-mcp

install-mcp:
	go install ./cmd/screener-pp-mcp

build-all: build build-mcp
