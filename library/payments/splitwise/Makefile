.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/splitwise-pp-cli$(BIN_EXT) ./cmd/splitwise-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/splitwise-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/splitwise-pp-mcp$(BIN_EXT) ./cmd/splitwise-pp-mcp

install-mcp:
	go install ./cmd/splitwise-pp-mcp

build-all: build build-mcp
