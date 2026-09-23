.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/zimmo-pp-cli$(BIN_EXT) ./cmd/zimmo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zimmo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/zimmo-pp-mcp$(BIN_EXT) ./cmd/zimmo-pp-mcp

install-mcp:
	go install ./cmd/zimmo-pp-mcp

build-all: build build-mcp
