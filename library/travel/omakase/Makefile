.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/omakase-pp-cli$(BIN_EXT) ./cmd/omakase-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/omakase-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/omakase-pp-mcp$(BIN_EXT) ./cmd/omakase-pp-mcp

install-mcp:
	go install ./cmd/omakase-pp-mcp

build-all: build build-mcp
