.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/ashby-pp-cli$(BIN_EXT) ./cmd/ashby-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ashby-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ashby-pp-mcp$(BIN_EXT) ./cmd/ashby-pp-mcp

install-mcp:
	go install ./cmd/ashby-pp-mcp

build-all: build build-mcp
