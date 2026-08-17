.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/bambu-pp-cli$(BIN_EXT) ./cmd/bambu-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bambu-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bambu-pp-mcp$(BIN_EXT) ./cmd/bambu-pp-mcp

install-mcp:
	go install ./cmd/bambu-pp-mcp

build-all: build build-mcp
