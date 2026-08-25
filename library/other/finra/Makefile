.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/finra-pp-cli$(BIN_EXT) ./cmd/finra-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/finra-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/finra-pp-mcp$(BIN_EXT) ./cmd/finra-pp-mcp

install-mcp:
	go install ./cmd/finra-pp-mcp

build-all: build build-mcp
