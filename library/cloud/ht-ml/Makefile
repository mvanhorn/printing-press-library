.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/ht-ml-pp-cli$(BIN_EXT) ./cmd/ht-ml-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ht-ml-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ht-ml-pp-mcp$(BIN_EXT) ./cmd/ht-ml-pp-mcp

install-mcp:
	go install ./cmd/ht-ml-pp-mcp

build-all: build build-mcp
