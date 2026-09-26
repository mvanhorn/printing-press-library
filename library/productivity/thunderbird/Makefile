.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/thunderbird-pp-cli$(BIN_EXT) ./cmd/thunderbird-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/thunderbird-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/thunderbird-pp-mcp$(BIN_EXT) ./cmd/thunderbird-pp-mcp

install-mcp:
	go install ./cmd/thunderbird-pp-mcp

build-all: build build-mcp
