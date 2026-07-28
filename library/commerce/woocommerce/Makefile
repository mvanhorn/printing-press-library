.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/woocommerce-pp-cli$(BIN_EXT) ./cmd/woocommerce-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/woocommerce-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/woocommerce-pp-mcp$(BIN_EXT) ./cmd/woocommerce-pp-mcp

install-mcp:
	go install ./cmd/woocommerce-pp-mcp

build-all: build build-mcp
