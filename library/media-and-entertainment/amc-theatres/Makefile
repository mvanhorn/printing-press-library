.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/amc-theatres-pp-cli$(BIN_EXT) ./cmd/amc-theatres-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/amc-theatres-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/amc-theatres-pp-mcp$(BIN_EXT) ./cmd/amc-theatres-pp-mcp

install-mcp:
	go install ./cmd/amc-theatres-pp-mcp

build-all: build build-mcp
