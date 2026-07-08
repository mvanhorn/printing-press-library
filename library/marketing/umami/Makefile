.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/umami-pp-cli$(BIN_EXT) ./cmd/umami-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/umami-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/umami-pp-mcp$(BIN_EXT) ./cmd/umami-pp-mcp

install-mcp:
	go install ./cmd/umami-pp-mcp

build-all: build build-mcp
