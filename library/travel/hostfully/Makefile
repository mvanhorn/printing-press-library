.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/hostfully-pp-cli$(BIN_EXT) ./cmd/hostfully-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/hostfully-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/hostfully-pp-mcp$(BIN_EXT) ./cmd/hostfully-pp-mcp

install-mcp:
	go install ./cmd/hostfully-pp-mcp

build-all: build build-mcp
