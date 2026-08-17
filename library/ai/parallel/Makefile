.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/parallel-pp-cli$(BIN_EXT) ./cmd/parallel-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/parallel-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/parallel-pp-mcp$(BIN_EXT) ./cmd/parallel-pp-mcp

install-mcp:
	go install ./cmd/parallel-pp-mcp

build-all: build build-mcp
