.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/ihatepdf-cv-pp-cli$(BIN_EXT) ./cmd/ihatepdf-cv-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ihatepdf-cv-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ihatepdf-cv-pp-mcp$(BIN_EXT) ./cmd/ihatepdf-cv-pp-mcp

install-mcp:
	go install ./cmd/ihatepdf-cv-pp-mcp

build-all: build build-mcp
