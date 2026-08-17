.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/janeapp-pp-cli$(BIN_EXT) ./cmd/janeapp-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/janeapp-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/janeapp-pp-mcp$(BIN_EXT) ./cmd/janeapp-pp-mcp

install-mcp:
	go install ./cmd/janeapp-pp-mcp

build-all: build build-mcp
