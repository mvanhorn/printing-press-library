.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/care-pp-cli$(BIN_EXT) ./cmd/care-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/care-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/care-pp-mcp$(BIN_EXT) ./cmd/care-pp-mcp

install-mcp:
	go install ./cmd/care-pp-mcp

build-all: build build-mcp
