.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/bland-pp-cli$(BIN_EXT) ./cmd/bland-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bland-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/bland-pp-mcp$(BIN_EXT) ./cmd/bland-pp-mcp

install-mcp:
	go install ./cmd/bland-pp-mcp

build-all: build build-mcp
