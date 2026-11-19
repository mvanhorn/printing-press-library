.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/etsy-pp-cli$(BIN_EXT) ./cmd/etsy-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/etsy-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/etsy-pp-mcp$(BIN_EXT) ./cmd/etsy-pp-mcp

install-mcp:
	go install ./cmd/etsy-pp-mcp

build-all: build build-mcp
