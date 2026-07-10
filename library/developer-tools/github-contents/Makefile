.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/github-contents-pp-cli$(BIN_EXT) ./cmd/github-contents-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/github-contents-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/github-contents-pp-mcp$(BIN_EXT) ./cmd/github-contents-pp-mcp

install-mcp:
	go install ./cmd/github-contents-pp-mcp

build-all: build build-mcp
