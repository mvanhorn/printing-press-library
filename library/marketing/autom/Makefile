.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/autom-pp-cli$(BIN_EXT) ./cmd/autom-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/autom-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/autom-pp-mcp$(BIN_EXT) ./cmd/autom-pp-mcp

install-mcp:
	go install ./cmd/autom-pp-mcp

build-all: build build-mcp
