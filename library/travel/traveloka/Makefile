.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/traveloka-pp-cli$(BIN_EXT) ./cmd/traveloka-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/traveloka-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/traveloka-pp-mcp$(BIN_EXT) ./cmd/traveloka-pp-mcp

install-mcp:
	go install ./cmd/traveloka-pp-mcp

build-all: build build-mcp
