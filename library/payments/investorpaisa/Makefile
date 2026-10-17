.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/investorpaisa-pp-cli$(BIN_EXT) ./cmd/investorpaisa-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/investorpaisa-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/investorpaisa-pp-mcp$(BIN_EXT) ./cmd/investorpaisa-pp-mcp

install-mcp:
	go install ./cmd/investorpaisa-pp-mcp

build-all: build build-mcp
