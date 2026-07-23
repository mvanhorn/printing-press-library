.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/zoho-campaigns-pp-cli$(BIN_EXT) ./cmd/zoho-campaigns-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zoho-campaigns-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zoho-campaigns-pp-mcp$(BIN_EXT) ./cmd/zoho-campaigns-pp-mcp

install-mcp:
	go install ./cmd/zoho-campaigns-pp-mcp

build-all: build build-mcp
