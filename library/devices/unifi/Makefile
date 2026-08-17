.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/unifi-pp-cli$(BIN_EXT) ./cmd/unifi-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/unifi-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/unifi-pp-mcp$(BIN_EXT) ./cmd/unifi-pp-mcp

install-mcp:
	go install ./cmd/unifi-pp-mcp

build-all: build build-mcp
