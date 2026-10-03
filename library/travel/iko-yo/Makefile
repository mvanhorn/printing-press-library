.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/iko-yo-pp-cli$(BIN_EXT) ./cmd/iko-yo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/iko-yo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/iko-yo-pp-mcp$(BIN_EXT) ./cmd/iko-yo-pp-mcp

install-mcp:
	go install ./cmd/iko-yo-pp-mcp

build-all: build build-mcp
