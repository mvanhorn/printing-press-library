.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/teslatracker-pp-cli$(BIN_EXT) ./cmd/teslatracker-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/teslatracker-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/teslatracker-pp-mcp$(BIN_EXT) ./cmd/teslatracker-pp-mcp

install-mcp:
	go install ./cmd/teslatracker-pp-mcp

build-all: build build-mcp
