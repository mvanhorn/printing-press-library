.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/agentmail-pp-cli$(BIN_EXT) ./cmd/agentmail-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/agentmail-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/agentmail-pp-mcp$(BIN_EXT) ./cmd/agentmail-pp-mcp

install-mcp:
	go install ./cmd/agentmail-pp-mcp

build-all: build build-mcp
