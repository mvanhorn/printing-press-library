.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/tabelog-pp-cli$(BIN_EXT) ./cmd/tabelog-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tabelog-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/tabelog-pp-mcp$(BIN_EXT) ./cmd/tabelog-pp-mcp

install-mcp:
	go install ./cmd/tabelog-pp-mcp

build-all: build build-mcp
