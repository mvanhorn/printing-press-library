.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/ticktick-pp-cli$(BIN_EXT) ./cmd/ticktick-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ticktick-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ticktick-pp-mcp$(BIN_EXT) ./cmd/ticktick-pp-mcp

install-mcp:
	go install ./cmd/ticktick-pp-mcp

build-all: build build-mcp
