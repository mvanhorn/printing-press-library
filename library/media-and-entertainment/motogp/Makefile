.PHONY: build test lint install clean

# Resolve the target GOOS from make's own variable so it honors both
# `GOOS=windows make build` (environment) and `make build GOOS=windows`
# (make override); `$(shell go env GOOS)` only saw the former and dropped the
# `.exe` suffix for the latter. Export it so the build recipe cross-compiles too.
GOOS ?= $(shell go env GOOS)
export GOOS
BIN_EXT := $(if $(filter windows,$(GOOS)),.exe,)

build:
	go build -o bin/motogp-pp-cli$(BIN_EXT) ./cmd/motogp-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/motogp-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/motogp-pp-mcp$(BIN_EXT) ./cmd/motogp-pp-mcp

install-mcp:
	go install ./cmd/motogp-pp-mcp

build-all: build build-mcp
