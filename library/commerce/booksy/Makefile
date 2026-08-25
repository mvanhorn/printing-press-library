.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/booksy-pp-cli$(BIN_EXT) ./cmd/booksy-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/booksy-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/booksy-pp-mcp$(BIN_EXT) ./cmd/booksy-pp-mcp

install-mcp:
	go install ./cmd/booksy-pp-mcp

build-all: build build-mcp
