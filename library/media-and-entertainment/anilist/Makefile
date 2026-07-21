.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/anilist-pp-cli$(BIN_EXT) ./cmd/anilist-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/anilist-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/anilist-pp-mcp$(BIN_EXT) ./cmd/anilist-pp-mcp

install-mcp:
	go install ./cmd/anilist-pp-mcp

build-all: build build-mcp
