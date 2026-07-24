.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/peekaboo-pp-cli$(BIN_EXT) ./cmd/peekaboo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/peekaboo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/peekaboo-pp-mcp$(BIN_EXT) ./cmd/peekaboo-pp-mcp

install-mcp:
	go install ./cmd/peekaboo-pp-mcp

build-all: build build-mcp
