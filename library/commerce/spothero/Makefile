.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/spothero-pp-cli$(BIN_EXT) ./cmd/spothero-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/spothero-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/spothero-pp-mcp$(BIN_EXT) ./cmd/spothero-pp-mcp

install-mcp:
	go install ./cmd/spothero-pp-mcp

build-all: build build-mcp
