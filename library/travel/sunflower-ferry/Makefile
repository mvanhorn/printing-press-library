.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/sunflower-ferry-pp-cli$(BIN_EXT) ./cmd/sunflower-ferry-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sunflower-ferry-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/sunflower-ferry-pp-mcp$(BIN_EXT) ./cmd/sunflower-ferry-pp-mcp

install-mcp:
	go install ./cmd/sunflower-ferry-pp-mcp

build-all: build build-mcp
