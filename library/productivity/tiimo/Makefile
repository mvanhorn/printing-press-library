.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/tiimo-pp-cli$(BIN_EXT) ./cmd/tiimo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tiimo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tiimo-pp-mcp$(BIN_EXT) ./cmd/tiimo-pp-mcp

install-mcp:
	go install ./cmd/tiimo-pp-mcp

build-all: build build-mcp
