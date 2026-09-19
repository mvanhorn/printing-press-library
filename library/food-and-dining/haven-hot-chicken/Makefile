.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/haven-hot-chicken-pp-cli$(BIN_EXT) ./cmd/haven-hot-chicken-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/haven-hot-chicken-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/haven-hot-chicken-pp-mcp$(BIN_EXT) ./cmd/haven-hot-chicken-pp-mcp

install-mcp:
	go install ./cmd/haven-hot-chicken-pp-mcp

build-all: build build-mcp
