.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/google-trends-pp-cli$(BIN_EXT) ./cmd/google-trends-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-trends-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/google-trends-pp-mcp$(BIN_EXT) ./cmd/google-trends-pp-mcp

install-mcp:
	go install ./cmd/google-trends-pp-mcp

build-all: build build-mcp
