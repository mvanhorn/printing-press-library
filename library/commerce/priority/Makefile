.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/priority-pp-cli$(BIN_EXT) ./cmd/priority-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/priority-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/priority-pp-mcp$(BIN_EXT) ./cmd/priority-pp-mcp

install-mcp:
	go install ./cmd/priority-pp-mcp

build-all: build build-mcp
