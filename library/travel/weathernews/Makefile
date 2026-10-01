.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/weathernews-pp-cli$(BIN_EXT) ./cmd/weathernews-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/weathernews-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/weathernews-pp-mcp$(BIN_EXT) ./cmd/weathernews-pp-mcp

install-mcp:
	go install ./cmd/weathernews-pp-mcp

build-all: build build-mcp
