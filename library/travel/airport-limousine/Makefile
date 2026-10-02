.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/airport-limousine-pp-cli$(BIN_EXT) ./cmd/airport-limousine-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/airport-limousine-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/airport-limousine-pp-mcp$(BIN_EXT) ./cmd/airport-limousine-pp-mcp

install-mcp:
	go install ./cmd/airport-limousine-pp-mcp

build-all: build build-mcp
