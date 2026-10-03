.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/nap-camp-pp-cli$(BIN_EXT) ./cmd/nap-camp-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nap-camp-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/nap-camp-pp-mcp$(BIN_EXT) ./cmd/nap-camp-pp-mcp

install-mcp:
	go install ./cmd/nap-camp-pp-mcp

build-all: build build-mcp
