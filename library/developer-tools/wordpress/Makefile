.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/wordpress-pp-cli$(BIN_EXT) ./cmd/wordpress-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/wordpress-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/wordpress-pp-mcp$(BIN_EXT) ./cmd/wordpress-pp-mcp

install-mcp:
	go install ./cmd/wordpress-pp-mcp

build-all: build build-mcp
