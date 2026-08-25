.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/discogs-pp-cli$(BIN_EXT) ./cmd/discogs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/discogs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/discogs-pp-mcp$(BIN_EXT) ./cmd/discogs-pp-mcp

install-mcp:
	go install ./cmd/discogs-pp-mcp

build-all: build build-mcp
