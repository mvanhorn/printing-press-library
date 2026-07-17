.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/starterstory-pp-cli$(BIN_EXT) ./cmd/starterstory-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/starterstory-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/starterstory-pp-mcp$(BIN_EXT) ./cmd/starterstory-pp-mcp

install-mcp:
	go install ./cmd/starterstory-pp-mcp

build-all: build build-mcp
