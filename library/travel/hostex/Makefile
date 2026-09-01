.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/hostex-pp-cli$(BIN_EXT) ./cmd/hostex-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/hostex-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/hostex-pp-mcp$(BIN_EXT) ./cmd/hostex-pp-mcp

install-mcp:
	go install ./cmd/hostex-pp-mcp

build-all: build build-mcp
