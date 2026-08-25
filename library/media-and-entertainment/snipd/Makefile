.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/snipd-pp-cli$(BIN_EXT) ./cmd/snipd-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/snipd-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/snipd-pp-mcp$(BIN_EXT) ./cmd/snipd-pp-mcp

install-mcp:
	go install ./cmd/snipd-pp-mcp

build-all: build build-mcp
