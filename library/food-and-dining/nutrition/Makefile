.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/nutrition-pp-cli$(BIN_EXT) ./cmd/nutrition-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nutrition-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/nutrition-pp-mcp$(BIN_EXT) ./cmd/nutrition-pp-mcp

install-mcp:
	go install ./cmd/nutrition-pp-mcp

build-all: build build-mcp
