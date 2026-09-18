.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/openbdap-pp-cli$(BIN_EXT) ./cmd/openbdap-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/openbdap-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/openbdap-pp-mcp$(BIN_EXT) ./cmd/openbdap-pp-mcp

install-mcp:
	go install ./cmd/openbdap-pp-mcp

build-all: build build-mcp
