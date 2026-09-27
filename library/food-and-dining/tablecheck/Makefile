.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/tablecheck-pp-cli$(BIN_EXT) ./cmd/tablecheck-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tablecheck-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/tablecheck-pp-mcp$(BIN_EXT) ./cmd/tablecheck-pp-mcp

install-mcp:
	go install ./cmd/tablecheck-pp-mcp

build-all: build build-mcp
