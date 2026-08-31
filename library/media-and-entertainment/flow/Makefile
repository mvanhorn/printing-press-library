.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/flow-pp-cli$(BIN_EXT) ./cmd/flow-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/flow-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/flow-pp-mcp$(BIN_EXT) ./cmd/flow-pp-mcp

install-mcp:
	go install ./cmd/flow-pp-mcp

build-all: build build-mcp
