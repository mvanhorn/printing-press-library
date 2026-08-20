.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/passage-pp-cli$(BIN_EXT) ./cmd/passage-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/passage-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/passage-pp-mcp$(BIN_EXT) ./cmd/passage-pp-mcp

install-mcp:
	go install ./cmd/passage-pp-mcp

build-all: build build-mcp
