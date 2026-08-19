.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/youtube-pp-cli$(BIN_EXT) ./cmd/youtube-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/youtube-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/youtube-pp-mcp$(BIN_EXT) ./cmd/youtube-pp-mcp

install-mcp:
	go install ./cmd/youtube-pp-mcp

build-all: build build-mcp
