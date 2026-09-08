.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/pbs-pp-cli$(BIN_EXT) ./cmd/pbs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pbs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pbs-pp-mcp$(BIN_EXT) ./cmd/pbs-pp-mcp

install-mcp:
	go install ./cmd/pbs-pp-mcp

build-all: build build-mcp
