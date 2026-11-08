.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/normattiva-pp-cli$(BIN_EXT) ./cmd/normattiva-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/normattiva-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/normattiva-pp-mcp$(BIN_EXT) ./cmd/normattiva-pp-mcp

install-mcp:
	go install ./cmd/normattiva-pp-mcp

build-all: build build-mcp
