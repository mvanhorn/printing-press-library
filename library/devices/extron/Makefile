.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/extron-pp-cli$(BIN_EXT) ./cmd/extron-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/extron-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/extron-pp-mcp$(BIN_EXT) ./cmd/extron-pp-mcp

install-mcp:
	go install ./cmd/extron-pp-mcp

build-all: build build-mcp
