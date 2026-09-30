.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/postmark-pp-cli$(BIN_EXT) ./cmd/postmark-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/postmark-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/postmark-pp-mcp$(BIN_EXT) ./cmd/postmark-pp-mcp

install-mcp:
	go install ./cmd/postmark-pp-mcp

build-all: build build-mcp
