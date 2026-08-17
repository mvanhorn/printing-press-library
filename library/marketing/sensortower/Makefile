.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/sensortower-pp-cli$(BIN_EXT) ./cmd/sensortower-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sensortower-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sensortower-pp-mcp$(BIN_EXT) ./cmd/sensortower-pp-mcp

install-mcp:
	go install ./cmd/sensortower-pp-mcp

build-all: build build-mcp
