.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/nccpl-pp-cli$(BIN_EXT) ./cmd/nccpl-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nccpl-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/nccpl-pp-mcp$(BIN_EXT) ./cmd/nccpl-pp-mcp

install-mcp:
	go install ./cmd/nccpl-pp-mcp

build-all: build build-mcp
