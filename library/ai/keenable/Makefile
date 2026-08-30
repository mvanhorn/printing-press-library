.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/keenable-pp-cli$(BIN_EXT) ./cmd/keenable-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/keenable-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/keenable-pp-mcp$(BIN_EXT) ./cmd/keenable-pp-mcp

install-mcp:
	go install ./cmd/keenable-pp-mcp

build-all: build build-mcp
