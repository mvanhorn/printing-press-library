.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/quranku-pp-cli$(BIN_EXT) ./cmd/quranku-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/quranku-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/quranku-pp-mcp$(BIN_EXT) ./cmd/quranku-pp-mcp

install-mcp:
	go install ./cmd/quranku-pp-mcp

build-all: build build-mcp
