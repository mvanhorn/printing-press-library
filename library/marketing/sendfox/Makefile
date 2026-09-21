.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/sendfox-pp-cli$(BIN_EXT) ./cmd/sendfox-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sendfox-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/sendfox-pp-mcp$(BIN_EXT) ./cmd/sendfox-pp-mcp

install-mcp:
	go install ./cmd/sendfox-pp-mcp

build-all: build build-mcp
