.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/fandango-pp-cli$(BIN_EXT) ./cmd/fandango-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/fandango-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/fandango-pp-mcp$(BIN_EXT) ./cmd/fandango-pp-mcp

install-mcp:
	go install ./cmd/fandango-pp-mcp

build-all: build build-mcp
