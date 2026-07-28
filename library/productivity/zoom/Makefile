.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/zoom-pp-cli$(BIN_EXT) ./cmd/zoom-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zoom-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zoom-pp-mcp$(BIN_EXT) ./cmd/zoom-pp-mcp

install-mcp:
	go install ./cmd/zoom-pp-mcp

build-all: build build-mcp
