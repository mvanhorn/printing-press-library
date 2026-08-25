.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/benzinga-pp-cli$(BIN_EXT) ./cmd/benzinga-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/benzinga-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/benzinga-pp-mcp$(BIN_EXT) ./cmd/benzinga-pp-mcp

install-mcp:
	go install ./cmd/benzinga-pp-mcp

build-all: build build-mcp
