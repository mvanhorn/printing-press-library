.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/inaturalist-pp-cli$(BIN_EXT) ./cmd/inaturalist-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/inaturalist-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/inaturalist-pp-mcp$(BIN_EXT) ./cmd/inaturalist-pp-mcp

install-mcp:
	go install ./cmd/inaturalist-pp-mcp

build-all: build build-mcp
