.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/immoweb-pp-cli$(BIN_EXT) ./cmd/immoweb-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/immoweb-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/immoweb-pp-mcp$(BIN_EXT) ./cmd/immoweb-pp-mcp

install-mcp:
	go install ./cmd/immoweb-pp-mcp

build-all: build build-mcp
