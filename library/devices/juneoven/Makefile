.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/juneoven-pp-cli$(BIN_EXT) ./cmd/juneoven-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/juneoven-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/juneoven-pp-mcp$(BIN_EXT) ./cmd/juneoven-pp-mcp

install-mcp:
	go install ./cmd/juneoven-pp-mcp

build-all: build build-mcp
