.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/dnsmadeeasy-pp-cli$(BIN_EXT) ./cmd/dnsmadeeasy-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/dnsmadeeasy-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/dnsmadeeasy-pp-mcp$(BIN_EXT) ./cmd/dnsmadeeasy-pp-mcp

install-mcp:
	go install ./cmd/dnsmadeeasy-pp-mcp

build-all: build build-mcp
