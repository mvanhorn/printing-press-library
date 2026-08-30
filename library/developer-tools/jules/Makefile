.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/jules-pp-cli$(BIN_EXT) ./cmd/jules-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jules-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/jules-pp-mcp$(BIN_EXT) ./cmd/jules-pp-mcp

install-mcp:
	go install ./cmd/jules-pp-mcp

build-all: build build-mcp
