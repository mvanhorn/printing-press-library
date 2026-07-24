.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/habitica-pp-cli$(BIN_EXT) ./cmd/habitica-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/habitica-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/habitica-pp-mcp$(BIN_EXT) ./cmd/habitica-pp-mcp

install-mcp:
	go install ./cmd/habitica-pp-mcp

build-all: build build-mcp
