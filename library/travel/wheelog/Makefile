.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/wheelog-pp-cli$(BIN_EXT) ./cmd/wheelog-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/wheelog-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/wheelog-pp-mcp$(BIN_EXT) ./cmd/wheelog-pp-mcp

install-mcp:
	go install ./cmd/wheelog-pp-mcp

build-all: build build-mcp
