.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/thepointsguy-pp-cli$(BIN_EXT) ./cmd/thepointsguy-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/thepointsguy-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/thepointsguy-pp-mcp$(BIN_EXT) ./cmd/thepointsguy-pp-mcp

install-mcp:
	go install ./cmd/thepointsguy-pp-mcp

build-all: build build-mcp
