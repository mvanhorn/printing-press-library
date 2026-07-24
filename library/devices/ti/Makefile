.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/ti-pp-cli$(BIN_EXT) ./cmd/ti-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ti-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ti-pp-mcp$(BIN_EXT) ./cmd/ti-pp-mcp

install-mcp:
	go install ./cmd/ti-pp-mcp

build-all: build build-mcp
