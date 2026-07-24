.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/movieglu-pp-cli$(BIN_EXT) ./cmd/movieglu-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/movieglu-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/movieglu-pp-mcp$(BIN_EXT) ./cmd/movieglu-pp-mcp

install-mcp:
	go install ./cmd/movieglu-pp-mcp

build-all: build build-mcp
