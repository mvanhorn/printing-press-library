.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/amazon-jobs-pp-cli$(BIN_EXT) ./cmd/amazon-jobs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/amazon-jobs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/amazon-jobs-pp-mcp$(BIN_EXT) ./cmd/amazon-jobs-pp-mcp

install-mcp:
	go install ./cmd/amazon-jobs-pp-mcp

build-all: build build-mcp
