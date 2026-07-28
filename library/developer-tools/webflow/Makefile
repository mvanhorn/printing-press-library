.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/webflow-pp-cli$(BIN_EXT) ./cmd/webflow-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/webflow-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/webflow-pp-mcp$(BIN_EXT) ./cmd/webflow-pp-mcp

install-mcp:
	go install ./cmd/webflow-pp-mcp

build-all: build build-mcp
