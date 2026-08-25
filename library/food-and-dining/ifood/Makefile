.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/ifood-pp-cli$(BIN_EXT) ./cmd/ifood-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ifood-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ifood-pp-mcp$(BIN_EXT) ./cmd/ifood-pp-mcp

install-mcp:
	go install ./cmd/ifood-pp-mcp

build-all: build build-mcp
