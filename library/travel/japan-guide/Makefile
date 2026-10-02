.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/japan-guide-pp-cli$(BIN_EXT) ./cmd/japan-guide-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/japan-guide-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/japan-guide-pp-mcp$(BIN_EXT) ./cmd/japan-guide-pp-mcp

install-mcp:
	go install ./cmd/japan-guide-pp-mcp

build-all: build build-mcp
