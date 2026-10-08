.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/dropbox-pp-cli$(BIN_EXT) ./cmd/dropbox-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/dropbox-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/dropbox-pp-mcp$(BIN_EXT) ./cmd/dropbox-pp-mcp

install-mcp:
	go install ./cmd/dropbox-pp-mcp

build-all: build build-mcp
