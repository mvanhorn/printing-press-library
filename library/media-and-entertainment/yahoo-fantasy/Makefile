.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/yahoo-fantasy-pp-cli$(BIN_EXT) ./cmd/yahoo-fantasy-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/yahoo-fantasy-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/yahoo-fantasy-pp-mcp$(BIN_EXT) ./cmd/yahoo-fantasy-pp-mcp

install-mcp:
	go install ./cmd/yahoo-fantasy-pp-mcp

build-all: build build-mcp
