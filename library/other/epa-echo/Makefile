.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/epa-echo-pp-cli$(BIN_EXT) ./cmd/epa-echo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/epa-echo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/epa-echo-pp-mcp$(BIN_EXT) ./cmd/epa-echo-pp-mcp

install-mcp:
	go install ./cmd/epa-echo-pp-mcp

build-all: build build-mcp
