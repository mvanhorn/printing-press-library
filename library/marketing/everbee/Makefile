.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/everbee-pp-cli$(BIN_EXT) ./cmd/everbee-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/everbee-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/everbee-pp-mcp$(BIN_EXT) ./cmd/everbee-pp-mcp

install-mcp:
	go install ./cmd/everbee-pp-mcp

build-all: build build-mcp
