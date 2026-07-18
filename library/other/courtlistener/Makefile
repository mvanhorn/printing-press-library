.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/courtlistener-pp-cli$(BIN_EXT) ./cmd/courtlistener-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/courtlistener-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/courtlistener-pp-mcp$(BIN_EXT) ./cmd/courtlistener-pp-mcp

install-mcp:
	go install ./cmd/courtlistener-pp-mcp

build-all: build build-mcp
