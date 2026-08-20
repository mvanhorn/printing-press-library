.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/3cx-xapi-pp-cli$(BIN_EXT) ./cmd/3cx-xapi-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/3cx-xapi-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/3cx-xapi-pp-mcp$(BIN_EXT) ./cmd/3cx-xapi-pp-mcp

install-mcp:
	go install ./cmd/3cx-xapi-pp-mcp

build-all: build build-mcp
