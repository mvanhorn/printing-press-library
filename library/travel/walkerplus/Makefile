.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/walkerplus-pp-cli$(BIN_EXT) ./cmd/walkerplus-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/walkerplus-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/walkerplus-pp-mcp$(BIN_EXT) ./cmd/walkerplus-pp-mcp

install-mcp:
	go install ./cmd/walkerplus-pp-mcp

build-all: build build-mcp
