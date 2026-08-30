.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/weaviate-collections-pp-cli$(BIN_EXT) ./cmd/weaviate-collections-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/weaviate-collections-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/weaviate-collections-pp-mcp$(BIN_EXT) ./cmd/weaviate-collections-pp-mcp

install-mcp:
	go install ./cmd/weaviate-collections-pp-mcp

build-all: build build-mcp
