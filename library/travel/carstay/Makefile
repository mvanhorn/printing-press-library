.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/carstay-pp-cli$(BIN_EXT) ./cmd/carstay-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/carstay-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/carstay-pp-mcp$(BIN_EXT) ./cmd/carstay-pp-mcp

install-mcp:
	go install ./cmd/carstay-pp-mcp

build-all: build build-mcp
