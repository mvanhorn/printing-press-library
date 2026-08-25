.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/foodpanda-pp-cli$(BIN_EXT) ./cmd/foodpanda-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/foodpanda-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/foodpanda-pp-mcp$(BIN_EXT) ./cmd/foodpanda-pp-mcp

install-mcp:
	go install ./cmd/foodpanda-pp-mcp

build-all: build build-mcp
