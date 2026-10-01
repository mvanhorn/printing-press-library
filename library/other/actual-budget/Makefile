.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/actual-budget-pp-cli$(BIN_EXT) ./cmd/actual-budget-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/actual-budget-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/actual-budget-pp-mcp$(BIN_EXT) ./cmd/actual-budget-pp-mcp

install-mcp:
	go install ./cmd/actual-budget-pp-mcp

build-all: build build-mcp
