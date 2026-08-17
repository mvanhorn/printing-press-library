.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/sea-airport-parking-pp-cli$(BIN_EXT) ./cmd/sea-airport-parking-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sea-airport-parking-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sea-airport-parking-pp-mcp$(BIN_EXT) ./cmd/sea-airport-parking-pp-mcp

install-mcp:
	go install ./cmd/sea-airport-parking-pp-mcp

build-all: build build-mcp
