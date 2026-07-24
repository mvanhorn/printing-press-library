.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/arcgis-pp-cli$(BIN_EXT) ./cmd/arcgis-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/arcgis-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/arcgis-pp-mcp$(BIN_EXT) ./cmd/arcgis-pp-mcp

install-mcp:
	go install ./cmd/arcgis-pp-mcp

build-all: build build-mcp
