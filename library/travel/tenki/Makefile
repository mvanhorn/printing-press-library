.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/tenki-pp-cli$(BIN_EXT) ./cmd/tenki-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tenki-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/tenki-pp-mcp$(BIN_EXT) ./cmd/tenki-pp-mcp

install-mcp:
	go install ./cmd/tenki-pp-mcp

build-all: build build-mcp
