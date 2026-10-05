.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/uber-jobs-pp-cli$(BIN_EXT) ./cmd/uber-jobs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/uber-jobs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/uber-jobs-pp-mcp$(BIN_EXT) ./cmd/uber-jobs-pp-mcp

install-mcp:
	go install ./cmd/uber-jobs-pp-mcp

build-all: build build-mcp
