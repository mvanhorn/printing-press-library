.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/myanimelist-pp-cli$(BIN_EXT) ./cmd/myanimelist-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/myanimelist-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/myanimelist-pp-mcp$(BIN_EXT) ./cmd/myanimelist-pp-mcp

install-mcp:
	go install ./cmd/myanimelist-pp-mcp

build-all: build build-mcp
