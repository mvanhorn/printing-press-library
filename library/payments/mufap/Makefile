.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/mufap-pp-cli$(BIN_EXT) ./cmd/mufap-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mufap-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/mufap-pp-mcp$(BIN_EXT) ./cmd/mufap-pp-mcp

install-mcp:
	go install ./cmd/mufap-pp-mcp

build-all: build build-mcp
