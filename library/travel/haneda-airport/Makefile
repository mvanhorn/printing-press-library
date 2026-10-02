.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/haneda-airport-pp-cli$(BIN_EXT) ./cmd/haneda-airport-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/haneda-airport-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/haneda-airport-pp-mcp$(BIN_EXT) ./cmd/haneda-airport-pp-mcp

install-mcp:
	go install ./cmd/haneda-airport-pp-mcp

build-all: build build-mcp
