.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/toyota-rentacar-pp-cli$(BIN_EXT) ./cmd/toyota-rentacar-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/toyota-rentacar-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/toyota-rentacar-pp-mcp$(BIN_EXT) ./cmd/toyota-rentacar-pp-mcp

install-mcp:
	go install ./cmd/toyota-rentacar-pp-mcp

build-all: build build-mcp
