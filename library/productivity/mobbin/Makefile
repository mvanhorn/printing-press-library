.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/mobbin-pp-cli$(BIN_EXT) ./cmd/mobbin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mobbin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/mobbin-pp-mcp$(BIN_EXT) ./cmd/mobbin-pp-mcp

install-mcp:
	go install ./cmd/mobbin-pp-mcp

build-all: build build-mcp
