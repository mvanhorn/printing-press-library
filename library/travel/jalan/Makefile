.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/jalan-pp-cli$(BIN_EXT) ./cmd/jalan-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jalan-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/jalan-pp-mcp$(BIN_EXT) ./cmd/jalan-pp-mcp

install-mcp:
	go install ./cmd/jalan-pp-mcp

build-all: build build-mcp
