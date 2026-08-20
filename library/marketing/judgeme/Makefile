.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/judgeme-pp-cli$(BIN_EXT) ./cmd/judgeme-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/judgeme-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/judgeme-pp-mcp$(BIN_EXT) ./cmd/judgeme-pp-mcp

install-mcp:
	go install ./cmd/judgeme-pp-mcp

build-all: build build-mcp
