.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/lever-pp-cli$(BIN_EXT) ./cmd/lever-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/lever-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/lever-pp-mcp$(BIN_EXT) ./cmd/lever-pp-mcp

install-mcp:
	go install ./cmd/lever-pp-mcp

build-all: build build-mcp
