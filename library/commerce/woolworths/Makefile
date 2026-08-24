.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/woolworths-pp-cli$(BIN_EXT) ./cmd/woolworths-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/woolworths-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/woolworths-pp-mcp$(BIN_EXT) ./cmd/woolworths-pp-mcp

install-mcp:
	go install ./cmd/woolworths-pp-mcp

build-all: build build-mcp
