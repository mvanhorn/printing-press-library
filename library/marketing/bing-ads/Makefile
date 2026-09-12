.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/bing-ads-pp-cli$(BIN_EXT) ./cmd/bing-ads-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bing-ads-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bing-ads-pp-mcp$(BIN_EXT) ./cmd/bing-ads-pp-mcp

install-mcp:
	go install ./cmd/bing-ads-pp-mcp

build-all: build build-mcp
