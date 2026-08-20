.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/netlify-pp-cli$(BIN_EXT) ./cmd/netlify-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/netlify-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/netlify-pp-mcp$(BIN_EXT) ./cmd/netlify-pp-mcp

install-mcp:
	go install ./cmd/netlify-pp-mcp

build-all: build build-mcp
