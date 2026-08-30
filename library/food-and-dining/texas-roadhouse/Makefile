.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/texas-roadhouse-pp-cli$(BIN_EXT) ./cmd/texas-roadhouse-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/texas-roadhouse-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/texas-roadhouse-pp-mcp$(BIN_EXT) ./cmd/texas-roadhouse-pp-mcp

install-mcp:
	go install ./cmd/texas-roadhouse-pp-mcp

build-all: build build-mcp
