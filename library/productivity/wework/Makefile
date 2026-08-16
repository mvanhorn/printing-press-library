.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/wework-pp-cli$(BIN_EXT) ./cmd/wework-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/wework-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/wework-pp-mcp$(BIN_EXT) ./cmd/wework-pp-mcp

install-mcp:
	go install ./cmd/wework-pp-mcp

build-all: build build-mcp
