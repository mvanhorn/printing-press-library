.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/productive-pp-cli$(BIN_EXT) ./cmd/productive-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/productive-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/productive-pp-mcp$(BIN_EXT) ./cmd/productive-pp-mcp

install-mcp:
	go install ./cmd/productive-pp-mcp

build-all: build build-mcp
