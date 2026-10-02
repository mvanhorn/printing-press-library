.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/jr-east-status-pp-cli$(BIN_EXT) ./cmd/jr-east-status-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jr-east-status-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/jr-east-status-pp-mcp$(BIN_EXT) ./cmd/jr-east-status-pp-mcp

install-mcp:
	go install ./cmd/jr-east-status-pp-mcp

build-all: build build-mcp
