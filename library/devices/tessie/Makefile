.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/tessie-pp-cli$(BIN_EXT) ./cmd/tessie-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tessie-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tessie-pp-mcp$(BIN_EXT) ./cmd/tessie-pp-mcp

install-mcp:
	go install ./cmd/tessie-pp-mcp

build-all: build build-mcp
