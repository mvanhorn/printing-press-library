.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/solaredge-pp-cli$(BIN_EXT) ./cmd/solaredge-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/solaredge-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/solaredge-pp-mcp$(BIN_EXT) ./cmd/solaredge-pp-mcp

install-mcp:
	go install ./cmd/solaredge-pp-mcp

build-all: build build-mcp
