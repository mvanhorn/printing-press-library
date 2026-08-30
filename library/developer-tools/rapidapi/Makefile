.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/rapidapi-pp-cli$(BIN_EXT) ./cmd/rapidapi-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/rapidapi-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/rapidapi-pp-mcp$(BIN_EXT) ./cmd/rapidapi-pp-mcp

install-mcp:
	go install ./cmd/rapidapi-pp-mcp

build-all: build build-mcp
