.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/jimmy-johns-pp-cli$(BIN_EXT) ./cmd/jimmy-johns-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jimmy-johns-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/jimmy-johns-pp-mcp$(BIN_EXT) ./cmd/jimmy-johns-pp-mcp

install-mcp:
	go install ./cmd/jimmy-johns-pp-mcp

build-all: build build-mcp
