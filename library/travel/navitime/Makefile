.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/navitime-pp-cli$(BIN_EXT) ./cmd/navitime-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/navitime-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/navitime-pp-mcp$(BIN_EXT) ./cmd/navitime-pp-mcp

install-mcp:
	go install ./cmd/navitime-pp-mcp

build-all: build build-mcp
