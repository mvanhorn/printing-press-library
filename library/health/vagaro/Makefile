.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/vagaro-pp-cli$(BIN_EXT) ./cmd/vagaro-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/vagaro-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/vagaro-pp-mcp$(BIN_EXT) ./cmd/vagaro-pp-mcp

install-mcp:
	go install ./cmd/vagaro-pp-mcp

build-all: build build-mcp
