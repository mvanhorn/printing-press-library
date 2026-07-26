.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/overpass-pp-cli$(BIN_EXT) ./cmd/overpass-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/overpass-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/overpass-pp-mcp$(BIN_EXT) ./cmd/overpass-pp-mcp

install-mcp:
	go install ./cmd/overpass-pp-mcp

build-all: build build-mcp
