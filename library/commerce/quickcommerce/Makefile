.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/quickcommerce-pp-cli$(BIN_EXT) ./cmd/quickcommerce-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/quickcommerce-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/quickcommerce-pp-mcp$(BIN_EXT) ./cmd/quickcommerce-pp-mcp

install-mcp:
	go install ./cmd/quickcommerce-pp-mcp

build-all: build build-mcp
