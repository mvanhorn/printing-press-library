.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/asoview-pp-cli$(BIN_EXT) ./cmd/asoview-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/asoview-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/asoview-pp-mcp$(BIN_EXT) ./cmd/asoview-pp-mcp

install-mcp:
	go install ./cmd/asoview-pp-mcp

build-all: build build-mcp
