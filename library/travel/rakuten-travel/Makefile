.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/rakuten-travel-pp-cli$(BIN_EXT) ./cmd/rakuten-travel-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/rakuten-travel-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/rakuten-travel-pp-mcp$(BIN_EXT) ./cmd/rakuten-travel-pp-mcp

install-mcp:
	go install ./cmd/rakuten-travel-pp-mcp

build-all: build build-mcp
