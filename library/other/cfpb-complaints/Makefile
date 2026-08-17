.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/cfpb-complaints-pp-cli$(BIN_EXT) ./cmd/cfpb-complaints-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cfpb-complaints-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cfpb-complaints-pp-mcp$(BIN_EXT) ./cmd/cfpb-complaints-pp-mcp

install-mcp:
	go install ./cmd/cfpb-complaints-pp-mcp

build-all: build build-mcp
