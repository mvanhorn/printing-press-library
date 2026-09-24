.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/immovlan-pp-cli$(BIN_EXT) ./cmd/immovlan-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/immovlan-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/immovlan-pp-mcp$(BIN_EXT) ./cmd/immovlan-pp-mcp

install-mcp:
	go install ./cmd/immovlan-pp-mcp

build-all: build build-mcp
