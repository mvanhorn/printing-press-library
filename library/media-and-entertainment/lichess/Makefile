.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/lichess-pp-cli$(BIN_EXT) ./cmd/lichess-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/lichess-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/lichess-pp-mcp$(BIN_EXT) ./cmd/lichess-pp-mcp

install-mcp:
	go install ./cmd/lichess-pp-mcp

build-all: build build-mcp
