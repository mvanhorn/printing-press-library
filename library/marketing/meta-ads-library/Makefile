.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/meta-ads-library-pp-cli$(BIN_EXT) ./cmd/meta-ads-library-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/meta-ads-library-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/meta-ads-library-pp-mcp$(BIN_EXT) ./cmd/meta-ads-library-pp-mcp

install-mcp:
	go install ./cmd/meta-ads-library-pp-mcp

build-all: build build-mcp
