.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/garmin-pp-cli$(BIN_EXT) ./cmd/garmin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/garmin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/garmin-pp-mcp$(BIN_EXT) ./cmd/garmin-pp-mcp

install-mcp:
	go install ./cmd/garmin-pp-mcp

build-all: build build-mcp
