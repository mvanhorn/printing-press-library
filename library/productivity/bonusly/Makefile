.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/bonusly-pp-cli$(BIN_EXT) ./cmd/bonusly-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bonusly-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bonusly-pp-mcp$(BIN_EXT) ./cmd/bonusly-pp-mcp

install-mcp:
	go install ./cmd/bonusly-pp-mcp

build-all: build build-mcp
