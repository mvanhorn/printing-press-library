.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/tradingview-pp-cli$(BIN_EXT) ./cmd/tradingview-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tradingview-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tradingview-pp-mcp$(BIN_EXT) ./cmd/tradingview-pp-mcp

install-mcp:
	go install ./cmd/tradingview-pp-mcp

build-all: build build-mcp
