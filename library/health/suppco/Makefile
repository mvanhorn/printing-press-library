.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/suppco-pp-cli$(BIN_EXT) ./cmd/suppco-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/suppco-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/suppco-pp-mcp$(BIN_EXT) ./cmd/suppco-pp-mcp

install-mcp:
	go install ./cmd/suppco-pp-mcp

build-all: build build-mcp
