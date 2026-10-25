.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/apptweak-pp-cli$(BIN_EXT) ./cmd/apptweak-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/apptweak-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/apptweak-pp-mcp$(BIN_EXT) ./cmd/apptweak-pp-mcp

install-mcp:
	go install ./cmd/apptweak-pp-mcp

build-all: build build-mcp
