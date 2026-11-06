.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/cloro-pp-cli$(BIN_EXT) ./cmd/cloro-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cloro-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/cloro-pp-mcp$(BIN_EXT) ./cmd/cloro-pp-mcp

install-mcp:
	go install ./cmd/cloro-pp-mcp

build-all: build build-mcp
