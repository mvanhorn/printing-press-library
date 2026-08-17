.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	cd vzero && go build -o ../bin/v0-pp-cli$(BIN_EXT) ./cmd/v0-pp-cli

test:
	cd vzero && go test ./...

lint:
	cd vzero && golangci-lint run

install:
	cd vzero && go install ./cmd/v0-pp-cli

clean:
	rm -rf bin/

build-mcp:
	cd vzero && go build -o ../bin/v0-pp-mcp$(BIN_EXT) ./cmd/v0-pp-mcp

install-mcp:
	cd vzero && go install ./cmd/v0-pp-mcp

build-all: build build-mcp
