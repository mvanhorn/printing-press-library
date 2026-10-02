.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/hello-cycling-pp-cli$(BIN_EXT) ./cmd/hello-cycling-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/hello-cycling-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/hello-cycling-pp-mcp$(BIN_EXT) ./cmd/hello-cycling-pp-mcp

install-mcp:
	go install ./cmd/hello-cycling-pp-mcp

build-all: build build-mcp
