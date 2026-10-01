.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/eplus-pp-cli$(BIN_EXT) ./cmd/eplus-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/eplus-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/eplus-pp-mcp$(BIN_EXT) ./cmd/eplus-pp-mcp

install-mcp:
	go install ./cmd/eplus-pp-mcp

build-all: build build-mcp
