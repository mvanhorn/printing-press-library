.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/gmail-pp-cli$(BIN_EXT) ./cmd/gmail-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gmail-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gmail-pp-mcp$(BIN_EXT) ./cmd/gmail-pp-mcp

install-mcp:
	go install ./cmd/gmail-pp-mcp

build-all: build build-mcp
