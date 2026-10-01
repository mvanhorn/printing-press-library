.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/ecbo-cloak-pp-cli$(BIN_EXT) ./cmd/ecbo-cloak-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ecbo-cloak-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/ecbo-cloak-pp-mcp$(BIN_EXT) ./cmd/ecbo-cloak-pp-mcp

install-mcp:
	go install ./cmd/ecbo-cloak-pp-mcp

build-all: build build-mcp
