.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/fooda-pp-cli$(BIN_EXT) ./cmd/fooda-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/fooda-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/fooda-pp-mcp$(BIN_EXT) ./cmd/fooda-pp-mcp

install-mcp:
	go install ./cmd/fooda-pp-mcp

build-all: build build-mcp
