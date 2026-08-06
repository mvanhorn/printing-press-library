.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/catapult-openfield-connect-pp-cli$(BIN_EXT) ./cmd/catapult-openfield-connect-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/catapult-openfield-connect-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/catapult-openfield-connect-pp-mcp$(BIN_EXT) ./cmd/catapult-openfield-connect-pp-mcp

install-mcp:
	go install ./cmd/catapult-openfield-connect-pp-mcp

build-all: build build-mcp
