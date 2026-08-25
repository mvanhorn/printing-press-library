.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/intercom-pp-cli$(BIN_EXT) ./cmd/intercom-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/intercom-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/intercom-pp-mcp$(BIN_EXT) ./cmd/intercom-pp-mcp

install-mcp:
	go install ./cmd/intercom-pp-mcp

build-all: build build-mcp
