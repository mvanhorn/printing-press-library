.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/moneycontrol-pp-cli$(BIN_EXT) ./cmd/moneycontrol-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/moneycontrol-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/moneycontrol-pp-mcp$(BIN_EXT) ./cmd/moneycontrol-pp-mcp

install-mcp:
	go install ./cmd/moneycontrol-pp-mcp

build-all: build build-mcp
