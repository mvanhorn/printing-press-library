.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/bookclicker-pp-cli$(BIN_EXT) ./cmd/bookclicker-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bookclicker-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bookclicker-pp-mcp$(BIN_EXT) ./cmd/bookclicker-pp-mcp

install-mcp:
	go install ./cmd/bookclicker-pp-mcp

build-all: build build-mcp
